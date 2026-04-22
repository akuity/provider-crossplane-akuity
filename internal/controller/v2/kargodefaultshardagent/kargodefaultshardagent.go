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
// KargoDefaultShardAgent controller. It owns the single
// kargoInstanceSpec.defaultShardAgent field of a Kargo instance via
// the PatchKargoInstance endpoint, which keys by opaque Akuity ID and
// server-side-merges into only the provided sub-tree.
package kargodefaultshardagent

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/akuityio/provider-crossplane-akuity/internal/event"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	apisv1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// Setup registers the controller.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha2.KargoDefaultShardAgentGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

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
	defer base.PropagateObservedGeneration(mg)
	kargoID, err := e.resolveKargoID(ctx, mg)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ki, err := e.Client.GetKargoInstanceByID(ctx, kargoID)
	if err != nil {
		if reason.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		if reason.IsProvisioningWait(err) {
			mg.SetConditions(xpv1.Unavailable())
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
		}
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	observed := ki.GetSpec().GetDefaultShardAgent()
	mg.Status.AtProvider = v1alpha2.KargoDefaultShardAgentObservation{
		AgentName:       observed,
		KargoInstanceID: kargoID,
	}
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
	defer base.PropagateObservedGeneration(mg)
	if err := e.patch(ctx, mg, mg.Spec.ForProvider.AgentName); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.GetName())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)
	return managed.ExternalUpdate{}, e.patch(ctx, mg, mg.Spec.ForProvider.AgentName)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)
	// Clearing the default is the API-neutral "no pinning" signal — we
	// intentionally do not delete the Kargo instance itself.
	return managed.ExternalDelete{}, e.patch(ctx, mg, "")
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// patch writes the desired defaultShardAgent via the narrow
// PatchKargoInstance endpoint. No prior Get is needed — the server
// merges into only the provided sub-tree.
func (e *external) patch(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent, desired string) error {
	kargoID, err := e.resolveKargoID(ctx, mg)
	if err != nil {
		return err
	}

	patch, err := structpb.NewStruct(map[string]any{
		"spec": map[string]any{
			"kargoInstanceSpec": map[string]any{
				"defaultShardAgent": desired,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("build defaultShardAgent patch: %w", err)
	}

	return e.Client.PatchKargoInstance(ctx, kargoID, patch)
}

// resolveKargoID returns the opaque Akuity ID of the target Kargo
// instance. ForProvider.KargoInstanceID takes precedence; if absent,
// KargoInstanceRef is resolved against a KargoInstance MR in the same
// namespace and its Status.AtProvider.ID is used.
//
// The cached Status.AtProvider.KargoInstanceID is consulted only as a
// last-resort during deletion when the referenced KargoInstance MR has
// itself been removed (typical composition teardown). It is NOT used
// to paper over transient kube-apiserver errors or a pending new
// parent, since that would let a live ref retarget silently keep
// patching the previous Kargo instance.
func (e *external) resolveKargoID(ctx context.Context, mg *v1alpha2.KargoDefaultShardAgent) (string, error) {
	if id := mg.Spec.ForProvider.KargoInstanceID; id != "" {
		return id, nil
	}
	if mg.Spec.ForProvider.KargoInstanceRef == nil || mg.Spec.ForProvider.KargoInstanceRef.Name == "" {
		return "", fmt.Errorf("one of spec.forProvider.kargoInstanceId or spec.forProvider.kargoInstanceRef must be set")
	}
	ki := &v1alpha2.KargoInstance{}
	key := k8stypes.NamespacedName{Name: mg.Spec.ForProvider.KargoInstanceRef.Name, Namespace: mg.GetNamespace()}
	if err := e.Kube.Get(ctx, key, ki); err != nil {
		if apierrors.IsNotFound(err) && meta.WasDeleted(mg) {
			if cached := mg.Status.AtProvider.KargoInstanceID; cached != "" {
				return cached, nil
			}
		}
		return "", fmt.Errorf("could not resolve KargoInstanceRef %s/%s: %w", key.Namespace, key.Name, err)
	}
	if ki.Status.AtProvider.ID == "" {
		return "", fmt.Errorf("referenced KargoInstance %s/%s has not yet reported an ID; waiting for its controller to observe", key.Namespace, key.Name)
	}
	return ki.Status.AtProvider.ID, nil
}
