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
// controller. It owns the ipAllowList field of an Akuity ArgoCD
// Instance via the PatchInstance endpoint, which keys by opaque
// Akuity ID and server-side-merges into only the provided sub-tree.
// Other InstanceSpec fields are untouched, so this MR can coexist
// with an Instance MR that deliberately leaves ipAllowList unset.
package instanceipallowlist

import (
	"context"
	"fmt"
	"reflect"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/event"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

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
	recorder := event.NewRecorder(mgr, name)

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
	instanceID, err := e.resolveInstanceID(ctx, mg)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ai, err := e.Client.GetInstanceByID(ctx, instanceID)
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
	if err := e.patch(ctx, mg, mg.Spec.ForProvider.AllowList); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.GetName())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha2.InstanceIpAllowList) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, e.patch(ctx, mg, mg.Spec.ForProvider.AllowList)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha2.InstanceIpAllowList) (managed.ExternalDelete, error) {
	// Delete clears the allow list (empty list, not missing key) rather
	// than deleting the Instance itself. This assumes the MR exclusively
	// owns the instance's ipAllowList; mixed ownership must be modelled
	// via Composition, not co-reconciling MRs.
	return managed.ExternalDelete{}, e.patch(ctx, mg, nil)
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// patch writes the desired ipAllowList to the Akuity Instance via the
// narrow PatchInstance endpoint. No prior Get is needed — the server
// merges into only the provided sub-tree.
func (e *external) patch(ctx context.Context, mg *v1alpha2.InstanceIpAllowList, desired []*v1alpha2.IPAllowListEntry) error {
	instanceID, err := e.resolveInstanceID(ctx, mg)
	if err != nil {
		return err
	}

	ipAllowList := make([]any, 0, len(desired))
	for _, d := range desired {
		if d == nil {
			continue
		}
		entry := map[string]any{"ip": d.Ip}
		if d.Description != "" {
			entry["description"] = d.Description
		}
		ipAllowList = append(ipAllowList, entry)
	}

	patch, err := structpb.NewStruct(map[string]any{
		"spec": map[string]any{
			"ipAllowList": ipAllowList,
		},
	})
	if err != nil {
		return fmt.Errorf("build ipAllowList patch: %w", err)
	}

	return e.Client.PatchInstance(ctx, instanceID, patch)
}

// resolveInstanceID returns the opaque Akuity ID of the target Instance.
// ForProvider.InstanceID takes precedence; if absent, InstanceRef is
// resolved against an Instance MR in the same namespace and its
// Status.AtProvider.ID is used. A pending Instance (ID not yet reported)
// is surfaced as a reconcile error so controller-runtime requeues.
func (e *external) resolveInstanceID(ctx context.Context, mg *v1alpha2.InstanceIpAllowList) (string, error) {
	if id := mg.Spec.ForProvider.InstanceID; id != "" {
		return id, nil
	}
	if mg.Spec.ForProvider.InstanceRef == nil || mg.Spec.ForProvider.InstanceRef.Name == "" {
		return "", fmt.Errorf("one of spec.forProvider.instanceId or spec.forProvider.instanceRef must be set")
	}

	inst := &v1alpha2.Instance{}
	key := k8stypes.NamespacedName{Name: mg.Spec.ForProvider.InstanceRef.Name, Namespace: mg.GetNamespace()}
	if err := e.Kube.Get(ctx, key, inst); err != nil {
		return "", fmt.Errorf("could not resolve InstanceRef %s/%s: %w", key.Namespace, key.Name, err)
	}
	if inst.Status.AtProvider.ID == "" {
		return "", fmt.Errorf("referenced Instance %s/%s has not yet reported an ID; waiting for its controller to observe", key.Namespace, key.Name)
	}
	return inst.Status.AtProvider.ID, nil
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
