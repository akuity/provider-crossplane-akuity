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

// Package instanceipallowlist is the v1alpha2 InstanceIpAllowList
// controller. It manages only the ipAllowList field of an Akuity
// ArgoCD Instance — users opt into it by owning the allow list as a
// separate managed resource (typically paired with an Instance that
// does NOT set ipAllowList). The reconcile path fetches the current
// Instance spec, patches ipAllowList, and re-applies; other fields
// flow through untouched.
package instanceipallowlist

import (
	"context"
	"fmt"
	"reflect"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	"github.com/google/go-cmp/cmp"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	apisv1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// Setup registers the controller with the manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha2.InstanceIpAllowListGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorderFor(name))

	conn := &base.Connector[*v1alpha2.InstanceIpAllowList]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha2.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha2.InstanceIpAllowList] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.InstanceIpAllowListGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha2.InstanceIpAllowList](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.InstanceIpAllowList{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha2.InstanceIpAllowList) (managed.ExternalObservation, error) {
	_, instanceName, err := e.resolveInstance(ctx, mg)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ai, err := e.Client.GetInstance(ctx, instanceName)
	if err != nil {
		if reason.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	observed := pbEntriesToSpec(ai.GetSpec().GetIpAllowList())
	mg.Status.AtProvider = v1alpha2.InstanceIpAllowListObservation{AllowList: observed}
	mg.SetConditions(xpv1.Available())

	upToDate := reflect.DeepEqual(mg.Spec.ForProvider.AllowList, observed)
	if !upToDate {
		e.Logger.Debug("InstanceIpAllowList drift detected",
			"diff", cmp.Diff(mg.Spec.ForProvider.AllowList, observed))
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mg *v1alpha2.InstanceIpAllowList) (managed.ExternalCreation, error) {
	if err := e.apply(ctx, mg, mg.Spec.ForProvider.AllowList); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.GetName())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha2.InstanceIpAllowList) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, e.apply(ctx, mg, mg.Spec.ForProvider.AllowList)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha2.InstanceIpAllowList) (managed.ExternalDelete, error) {
	// Delete clears the allow list on the underlying Instance rather
	// than deleting the Instance itself. An empty slice is the
	// API-neutral "no restrictions" signal.
	return managed.ExternalDelete{}, e.apply(ctx, mg, nil)
}

func (e *external) Disconnect(ctx context.Context) error { return nil }

// apply fetches the current Instance spec, substitutes the IpAllowList
// field, and re-applies. Other InstanceSpec fields are preserved
// verbatim so this controller can coexist with an Instance MR that
// manages everything *except* the allow list.
func (e *external) apply(ctx context.Context, mg *v1alpha2.InstanceIpAllowList, desired []*v1alpha2.IPAllowListEntry) error {
	_, instanceName, err := e.resolveInstance(ctx, mg)
	if err != nil {
		return err
	}

	ai, err := e.Client.GetInstance(ctx, instanceName)
	if err != nil {
		return fmt.Errorf("could not get instance %q to patch IpAllowList: %w", instanceName, err)
	}

	// Build an ApplyInstanceRequest that only carries the argocd
	// section. ConfigMap/plugin fields are intentionally left nil —
	// Akuity treats nil-typed request fields as "unchanged".
	spec := ai.GetSpec()
	entries := make([]*argocdv1.IPAllowListEntry, 0, len(desired))
	for _, d := range desired {
		if d == nil {
			continue
		}
		entries = append(entries, &argocdv1.IPAllowListEntry{Ip: d.Ip, Description: d.Description})
	}

	spec.IpAllowList = entries
	ai.Spec = spec

	req := &argocdv1.ApplyInstanceRequest{
		IdType: idv1.Type_NAME,
		Id:     instanceName,
		// Passing the full (mutated) Instance back is not the shape
		// ApplyInstance accepts — see the Instance controller for how
		// v1alpha2.Instance maps to ApplyInstanceRequest. The v1alpha2
		// InstanceIpAllowList flow is intentionally a narrow patch:
		// we round-trip the observed protobuf Instance.Spec through
		// the structpb-wrapped Argocd payload the API expects.
	}
	argocdPB, err := argoCDPBFromProto(instanceName, ai)
	if err != nil {
		return err
	}
	req.Argocd = argocdPB

	return e.Client.ApplyInstance(ctx, req)
}

// resolveInstance returns (instanceID, instanceName) by looking up the
// referenced Instance MR in the same namespace. The ID may be empty if
// the Instance has not yet reported its AtProvider.ID; the Akuity apply
// and delete paths only need the name, so this is fine.
func (e *external) resolveInstance(ctx context.Context, mg *v1alpha2.InstanceIpAllowList) (string, string, error) {
	if mg.Spec.ForProvider.InstanceRef == nil || mg.Spec.ForProvider.InstanceRef.Name == "" {
		return "", "", fmt.Errorf("spec.forProvider.instanceRef.name must be set")
	}

	inst := &v1alpha2.Instance{}
	key := k8stypes.NamespacedName{Name: mg.Spec.ForProvider.InstanceRef.Name, Namespace: mg.GetNamespace()}
	if err := e.Kube.Get(ctx, key, inst); err != nil {
		return "", "", fmt.Errorf("could not resolve InstanceRef %s/%s: %w", key.Namespace, key.Name, err)
	}
	return inst.Status.AtProvider.ID, inst.Spec.ForProvider.Name, nil
}

func pbEntriesToSpec(in []*argocdv1.IPAllowListEntry) []*v1alpha2.IPAllowListEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]*v1alpha2.IPAllowListEntry, 0, len(in))
	for _, e := range in {
		if e == nil {
			continue
		}
		out = append(out, &v1alpha2.IPAllowListEntry{Ip: e.GetIp(), Description: e.GetDescription()})
	}
	return out
}
