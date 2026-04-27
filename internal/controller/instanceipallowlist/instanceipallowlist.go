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

// Package instanceipallowlist is the InstanceIpAllowList controller.
// It owns the ipAllowList field of an Akuity ArgoCD Instance via the
// PatchInstance endpoint, which keys by opaque
// Akuity ID and server-side-merges into only the provided sub-tree.
// Other InstanceSpec fields are untouched, so this MR can coexist
// with an Instance MR that deliberately leaves ipAllowList unset.
package instanceipallowlist

import (
	"context"
	"fmt"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"google.golang.org/protobuf/types/known/structpb"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/akuityio/provider-crossplane-akuity/internal/event"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// Setup registers the controller with the manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.InstanceIpAllowListGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

	conn := &base.Connector[*v1alpha1.InstanceIpAllowList]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewLegacyProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha1.InstanceIpAllowList] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.InstanceIpAllowListGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha1.InstanceIpAllowList](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.InstanceIpAllowList{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha1.InstanceIpAllowList) (managed.ExternalObservation, error) {
	defer base.PropagateObservedGeneration(mg)
	instanceID, err := e.resolveInstanceID(ctx, mg)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	// Short-circuit on a cached terminal write before any gateway round-
	// trip. With NameAsExternalName the external-name is stamped before
	// Create runs, so a Patch that fails terminally on bad input would
	// otherwise loop GetInstanceByID->Patch reject at controller-runtime
	// backoff (~2s).
	if e.HasTerminalWriteResource(mg, v1alpha1.InstanceIpAllowListGroupVersionKind) {
		if obs, err, ok := e.suppressTerminalWrite(mg, instanceID); ok {
			return obs, err
		}
	}
	if meta.GetExternalName(mg) == "" {
		return e.observeMissingExternalName(mg, instanceID)
	}

	ai, err := e.Client.GetInstanceByID(ctx, instanceID)
	if outcome, obs, rerr := base.ClassifyGetError(err); outcome != base.GetOK {
		return handleGetOutcome(mg, err, outcome, obs, rerr)
	}

	observed := pbEntriesToSpec(ai.GetSpec().GetIpAllowList())
	mg.Status.AtProvider = v1alpha1.InstanceIpAllowListObservation{
		AllowList:  observed,
		InstanceID: instanceID,
	}
	base.SetHealthCondition(mg, true)

	// Deletion fast-path: once the MR has a deletionTimestamp, Delete()
	// sends a Patch with an empty list, which the server applies
	// synchronously. On the next Observe the server-side list is
	// already empty, which is the post-delete state. Signaling
	// ResourceExists=false here lets the managed reconciler drop the
	// finalizer instead of re-entering Delete every poll. Without this
	// the MR spec still carries the last user-desired list, so the
	// drift compare below flips ResourceUpToDate=false and runtime
	// dispatches Delete on every reconcile for as long as the MR
	// lingers on the finalizer, issuing one PatchInstance per loop.
	if meta.WasDeleted(mg) && len(observed) == 0 {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// Drift is nil-vs-empty-sensitive: a user who writes
	// `allowList: []` and an API that returns nothing must resolve
	// equal, so we go through the shared DriftSpec (which applies
	// utilcmp.EquateEmpty) rather than reflect.DeepEqual.
	desired := mg.Spec.ForProvider.AllowList
	spec := driftSpec()
	upToDate, err := base.EvaluateDrift(ctx, spec, &desired, &observed, e.Logger, "InstanceIpAllowList")
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	if obs, err, ok := e.terminalGuardedObservation(mg, instanceID, upToDate); ok {
		return obs, err
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func handleGetOutcome(
	mg *v1alpha1.InstanceIpAllowList,
	err error,
	outcome base.GetOutcome,
	obs managed.ExternalObservation,
	rerr error,
) (managed.ExternalObservation, error) {
	switch outcome {
	case base.GetOK, base.GetAbsent:
		// GetOK is filtered by the caller; GetAbsent's pre-shaped
		// obs (ResourceExists=false) is returned as-is.
	case base.GetProvisioning:
		base.SetHealthCondition(mg, false)
	case base.GetTerminal:
		mg.SetConditions(xpv1.ReconcileError(err))
	}
	return obs, rerr
}

func (e *external) observeMissingExternalName(mg *v1alpha1.InstanceIpAllowList, instanceID string) (managed.ExternalObservation, error) {
	if obs, err, ok := e.suppressTerminalWrite(mg, instanceID); ok {
		return obs, err
	}
	return managed.ExternalObservation{ResourceExists: false}, nil
}

func (e *external) terminalGuardedObservation(mg *v1alpha1.InstanceIpAllowList, instanceID string, upToDate bool) (managed.ExternalObservation, error, bool) {
	if !upToDate {
		return e.suppressTerminalWrite(mg, instanceID)
	}
	e.clearTerminalWrite(mg, instanceID)
	return managed.ExternalObservation{}, nil, false
}

func (e *external) Create(ctx context.Context, mg *v1alpha1.InstanceIpAllowList) (managed.ExternalCreation, error) {
	defer base.PropagateObservedGeneration(mg)
	if err := e.patch(ctx, mg, mg.Spec.ForProvider.AllowList); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.GetName())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha1.InstanceIpAllowList) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)
	return managed.ExternalUpdate{}, e.patch(ctx, mg, mg.Spec.ForProvider.AllowList)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha1.InstanceIpAllowList) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)
	e.ClearTerminalWriteResource(mg, v1alpha1.InstanceIpAllowListGroupVersionKind)

	// Delete clears the allow list (empty list, not missing key) rather
	// than deleting the Instance itself. This assumes the MR exclusively
	// owns the instance's ipAllowList; mixed ownership must be modelled
	// via Composition, not co-reconciling MRs.
	return managed.ExternalDelete{}, e.patch(ctx, mg, nil)
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// patch writes the desired ipAllowList to the Akuity Instance via the
// narrow PatchInstance endpoint. No prior Get is needed; the server
// merges into only the provided sub-tree.
func (e *external) patch(ctx context.Context, mg *v1alpha1.InstanceIpAllowList, desired []*crossplanetypes.IPAllowListEntry) error {
	instanceID, err := e.resolveInstanceID(ctx, mg)
	if err != nil {
		return err
	}
	key, err := instanceIPAllowListTerminalWriteKey(mg, instanceID, desired)
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

	if err := e.Client.PatchInstance(ctx, instanceID, patch); err != nil {
		err = reason.ClassifyApplyError(err)
		if meta.WasDeleted(mg) {
			return err
		}
		return e.RecordTerminalWrite(key, err)
	}
	if !meta.WasDeleted(mg) {
		e.ClearTerminalWrite(key)
	}
	return nil
}

func (e *external) suppressTerminalWrite(mg *v1alpha1.InstanceIpAllowList, instanceID string) (managed.ExternalObservation, error, bool) {
	if e.TerminalWrites == nil {
		return managed.ExternalObservation{}, nil, false
	}
	key, err := instanceIPAllowListTerminalWriteKey(mg, instanceID, mg.Spec.ForProvider.AllowList)
	if err != nil {
		return e.SkipTerminalWriteGuard(err)
	}
	return e.SuppressTerminalWrite(mg, key)
}

func instanceIPAllowListTerminalWriteKey(mg *v1alpha1.InstanceIpAllowList, instanceID string, desired []*crossplanetypes.IPAllowListEntry) (base.TerminalWriteKey, error) {
	return base.NewTerminalWriteKey(mg, v1alpha1.InstanceIpAllowListGroupVersionKind, instanceID, desired)
}

func (e *external) clearTerminalWrite(mg *v1alpha1.InstanceIpAllowList, instanceID string) {
	if !e.HasTerminalWriteResource(mg, v1alpha1.InstanceIpAllowListGroupVersionKind) {
		return
	}
	key, err := instanceIPAllowListTerminalWriteKey(mg, instanceID, mg.Spec.ForProvider.AllowList)
	if err != nil {
		e.LogTerminalWriteGuardSkipped(err)
		return
	}
	e.ClearTerminalWrite(key)
}

// resolveInstanceID returns the opaque Akuity ID of the target Instance.
// ForProvider.InstanceID takes precedence; if absent, InstanceRef is
// resolved against an Instance MR in the same namespace and its
// Status.AtProvider.ID is used.
//
// The cached Status.AtProvider.InstanceID is consulted only as a
// last-resort during deletion when the referenced Instance MR has
// itself been removed (typical composition teardown). It is NOT used
// to paper over transient kube-apiserver errors or a pending new
// parent, since that would let a live ref retarget silently keep
// patching the previous Akuity Instance.
func (e *external) resolveInstanceID(ctx context.Context, mg *v1alpha1.InstanceIpAllowList) (string, error) {
	if id := mg.Spec.ForProvider.InstanceID; id != "" {
		return id, nil
	}
	if mg.Spec.ForProvider.InstanceRef == nil || mg.Spec.ForProvider.InstanceRef.Name == "" {
		return "", fmt.Errorf("one of spec.forProvider.instanceId or spec.forProvider.instanceRef must be set")
	}

	inst := &v1alpha1.Instance{}
	key := k8stypes.NamespacedName{Name: mg.Spec.ForProvider.InstanceRef.Name, Namespace: mg.GetNamespace()}
	if err := e.Kube.Get(ctx, key, inst); err != nil {
		if apierrors.IsNotFound(err) && meta.WasDeleted(mg) {
			if cached := mg.Status.AtProvider.InstanceID; cached != "" {
				return cached, nil
			}
		}
		return "", fmt.Errorf("could not resolve InstanceRef %s/%s: %w", key.Namespace, key.Name, err)
	}
	if inst.Status.AtProvider.ID == "" {
		return "", fmt.Errorf("referenced Instance %s/%s has not yet reported an ID; waiting for its controller to observe", key.Namespace, key.Name)
	}
	return inst.Status.AtProvider.ID, nil
}

// driftSpec is the resource's drift-detection recipe: a straight
// cmp.Equal on the allow-list slice, with the shared EquateEmpty
// baseline that makes nil-vs-empty-slice resolve equal.
func driftSpec() base.DriftSpec[[]*crossplanetypes.IPAllowListEntry] {
	return base.DriftSpec[[]*crossplanetypes.IPAllowListEntry]{}
}

func pbEntriesToSpec(in []*argocdv1.IPAllowListEntry) []*crossplanetypes.IPAllowListEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]*crossplanetypes.IPAllowListEntry, 0, len(in))
	for _, e := range in {
		if e == nil {
			continue
		}
		out = append(out, &crossplanetypes.IPAllowListEntry{Ip: e.GetIp(), Description: e.GetDescription()})
	}
	return out
}
