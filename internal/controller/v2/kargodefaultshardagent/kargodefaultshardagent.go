/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package kargodefaultshardagent is the v1alpha2
// KargoDefaultShardAgent controller. It manages the single
// kargoInstanceSpec.defaultShardAgent field of a Kargo instance by
// round-tripping GetKargoInstance + ApplyKargoInstance with only that
// field patched; other KargoInstanceSpec fields are preserved verbatim.
package kargodefaultshardagent

import (
	"context"
	"fmt"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"google.golang.org/protobuf/types/known/structpb"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	apisv1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// Setup registers the controller.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha2.KargoDefaultShardAgentGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorderFor(name))

	conn := &base.Connector[*v1alpha2.KargoDefaultShardAgent]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha2.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha2.KargoDefaultShardAgent] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.KargoDefaultShardAgentGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha2.KargoDefaultShardAgent](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.KargoDefaultShardAgent{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent) (managed.ExternalObservation, error) {
	_, kargoName, err := e.resolveKargo(ctx, mg)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ki, err := e.Client.GetKargoInstance(ctx, kargoName)
	if err != nil {
		if reason.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	observed := ki.GetSpec().GetDefaultShardAgent()
	mg.Status.AtProvider = v1alpha2.KargoDefaultShardAgentObservation{AgentName: observed}
	mg.SetConditions(xpv1.Available())

	upToDate := observed == mg.Spec.ForProvider.AgentName
	if !upToDate {
		e.Logger.Debug("KargoDefaultShardAgent drift detected",
			"observed", observed, "desired", mg.Spec.ForProvider.AgentName)
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent) (managed.ExternalCreation, error) {
	if err := e.apply(ctx, mg, mg.Spec.ForProvider.AgentName); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.GetName())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, e.apply(ctx, mg, mg.Spec.ForProvider.AgentName)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent) (managed.ExternalDelete, error) {
	// Clearing the default is the API-neutral "no pinning" signal —
	// we intentionally do not delete the Kargo instance.
	return managed.ExternalDelete{}, e.apply(ctx, mg, "")
}

func (e *external) Disconnect(ctx context.Context) error { return nil }

// apply fetches the current Kargo instance spec, patches
// DefaultShardAgent, and re-applies. Other instance-spec fields are
// carried through verbatim.
func (e *external) apply(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent, desired string) error {
	_, kargoName, err := e.resolveKargo(ctx, mg)
	if err != nil {
		return err
	}

	ki, err := e.Client.GetKargoInstance(ctx, kargoName)
	if err != nil {
		return fmt.Errorf("could not get kargo instance %q to patch defaultShardAgent: %w", kargoName, err)
	}

	if ki.Spec == nil {
		ki.Spec = &kargov1.KargoInstanceSpec{}
	}
	ki.Spec.DefaultShardAgent = desired

	// Re-wrap the KargoInstance into the on-wire `kargo` struct
	// ApplyKargoInstance expects. Going through protojson → structpb
	// is the same round-trip used elsewhere in this package.
	kargoPB, err := pbSpecToKargoStruct(ki)
	if err != nil {
		return err
	}

	return e.Client.ApplyKargoInstance(ctx, &kargov1.ApplyKargoInstanceRequest{
		IdType: idv1.Type_NAME,
		Id:     kargoName,
		Kargo:  kargoPB,
	})
}

// resolveKargo returns (kargoInstanceID, kargoInstanceName) by looking
// up the referenced KargoInstance MR in the same namespace. The ID may
// be empty if the KargoInstance has not yet reported its AtProvider.ID;
// the Akuity apply path only needs the name, so this is fine.
func (e *external) resolveKargo(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent) (string, string, error) {
	if mg.Spec.ForProvider.KargoInstanceRef == nil || mg.Spec.ForProvider.KargoInstanceRef.Name == "" {
		return "", "", fmt.Errorf("spec.forProvider.kargoInstanceRef.name must be set")
	}
	ki := &v1alpha2.KargoInstance{}
	key := k8stypes.NamespacedName{Name: mg.Spec.ForProvider.KargoInstanceRef.Name, Namespace: mg.GetNamespace()}
	if err := e.Kube.Get(ctx, key, ki); err != nil {
		return "", "", fmt.Errorf("could not resolve KargoInstanceRef %s/%s: %w", key.Namespace, key.Name, err)
	}
	return ki.Status.AtProvider.ID, ki.Spec.ForProvider.Name, nil
}

// pbSpecToKargoStruct wraps the mutated *kargov1.KargoInstance back
// into the on-wire `kargo` struct ApplyKargoInstance expects. The
// wire Kargo object is { kind, apiVersion, metadata.name, spec:
// KargoSpec{ description, version, fqdn, subdomain, oidcConfig,
// kargoInstanceSpec } } where kargoInstanceSpec is the protobuf
// KargoInstanceSpec payload carried directly on KargoInstance.Spec.
func pbSpecToKargoStruct(ki *kargov1.KargoInstance) (*structpb.Struct, error) {
	instanceSpec, err := marshal.ProtoToMap(ki.GetSpec())
	if err != nil {
		return nil, fmt.Errorf("encode KargoInstanceSpec: %w", err)
	}
	spec := map[string]any{
		"description":       ki.GetDescription(),
		"version":           ki.GetVersion(),
		"fqdn":              ki.GetFqdn(),
		"subdomain":         ki.GetSubdomain(),
		"kargoInstanceSpec": instanceSpec,
	}
	if oidc := ki.GetOidcConfig(); oidc != nil {
		oidcMap, err := marshal.ProtoToMap(oidc)
		if err != nil {
			return nil, fmt.Errorf("encode KargoOidcConfig: %w", err)
		}
		spec["oidcConfig"] = oidcMap
	}
	wrapper := map[string]any{
		"kind":       "Kargo",
		"apiVersion": "kargo.akuity.io/v1alpha1",
		"metadata":   map[string]any{"name": ki.GetName()},
		"spec":       spec,
	}
	s, err := structpb.NewStruct(wrapper)
	if err != nil {
		return nil, fmt.Errorf("new kargo struct: %w", err)
	}
	return s, nil
}
