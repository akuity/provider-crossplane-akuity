/*
Copyright 2022 The Crossplane Authors.

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

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/kube"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/event"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	generated "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/pointer"
)

const errTransformCluster = "cannot transform cluster to Akuity API model"

// Setup adds a controller that reconciles Cluster managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ClusterGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

	conn := &base.Connector[*v1alpha1.Cluster]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewLegacyProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha1.Cluster] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ClusterGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha1.Cluster](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Cluster{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha1.Cluster) (managed.ExternalObservation, error) { //nolint:gocyclo
	defer base.PropagateObservedGeneration(mg)

	instanceID, err := e.getInstanceID(ctx, mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider.InstanceRef)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	mg.Spec.ForProvider.InstanceID = instanceID

	// Short-circuit on a cached terminal write before any gateway round-
	// trip. With the NameAsExternalName initializer the external-name
	// is stamped before the first Create, so a Create that fails
	// terminally (instance ID not found, malformed Kustomization, bad
	// kubeconfig secret) would otherwise loop GetCluster->NotFound->
	// Create->reject at controller-runtime backoff (~2s).
	if e.HasTerminalWriteResource(mg, v1alpha1.ClusterGroupVersionKind) {
		if obs, err, ok := e.suppressTerminalWrite(mg); ok {
			return obs, err
		}
	}

	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// GetCluster is the source of truth for observation data such as
	// health, reconciliation, agent state, agent size, and target
	// version. Those fields do not round-trip through Export.
	akuityCluster, err := e.Client.GetCluster(ctx, instanceID, meta.GetExternalName(mg))
	if outcome, obs, rerr := base.ClassifyGetError(err); outcome != base.GetOK {
		switch outcome {
		case base.GetOK, base.GetAbsent:
			// GetOK is filtered by the enclosing `if`; GetAbsent's
			// pre-shaped obs (ResourceExists=false) is returned as-is.
		case base.GetProvisioning:
			base.SetHealthCondition(mg, false)
		case base.GetTerminal:
			mg.SetConditions(xpv1.ReconcileError(err))
		}
		return obs, rerr
	}

	actualCluster, err := APIToSpec(instanceID, mg.Spec.ForProvider, akuityCluster)
	if err != nil {
		newErr := fmt.Errorf("could not transform cluster from Akuity API: %w", err)
		mg.SetConditions(xpv1.ReconcileError(newErr))
		return managed.ExternalObservation{}, newErr
	}

	lateInitializeCluster(&mg.Spec.ForProvider, actualCluster)

	clusterObservation, err := observation.Cluster(akuityCluster)
	if err != nil {
		newErr := fmt.Errorf("could not transform cluster observation: %w", err)
		mg.SetConditions(xpv1.ReconcileError(newErr))
		return managed.ExternalObservation{}, newErr
	}

	mg.Status.AtProvider = clusterObservation
	base.SetHealthCondition(mg, clusterObservation.HealthStatus.Code == 1)

	// Drift compares against ExportInstanceByID's round-trippable spec,
	// the same structural shape ApplyInstance sends. If Export succeeds
	// but the cluster is missing from its Clusters list, fall back to
	// the GetCluster-derived spec because GetCluster already proved the
	// cluster exists.
	driftTarget, found, err := e.exportedClusterSpec(ctx, instanceID, meta.GetExternalName(mg), mg.Spec.ForProvider)
	if err != nil {
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}
	if !found {
		driftTarget = actualCluster
	}
	statusClusterSpec := driftTarget.ClusterSpec

	// MaintenanceMode and MaintenanceModeExpiry use the dedicated
	// set-maintenance-mode RPC, not ApplyInstance. ExportInstance echoes
	// them, and when the user enables maintenance without pinning an
	// expiry, the platform stamps a rolling expiry. Comparing user-nil
	// against server-stamped expiry would flap drift forever.
	//
	// Branch on user intent: when the user pins a field, compare against
	// the GetCluster value so real drift surfaces. When the user is
	// silent, force driftTarget back to nil so a server-stamped value
	// does not look like user-controlled drift.
	desiredData := mg.Spec.ForProvider.ClusterSpec.Data
	if desiredData.MaintenanceMode != nil {
		driftTarget.ClusterSpec.Data.MaintenanceMode = actualCluster.ClusterSpec.Data.MaintenanceMode
	} else {
		driftTarget.ClusterSpec.Data.MaintenanceMode = nil
	}
	if desiredData.MaintenanceModeExpiry != nil {
		driftTarget.ClusterSpec.Data.MaintenanceModeExpiry = actualCluster.ClusterSpec.Data.MaintenanceModeExpiry
	} else {
		driftTarget.ClusterSpec.Data.MaintenanceModeExpiry = nil
	}
	statusClusterSpec.Data.MaintenanceMode = actualCluster.ClusterSpec.Data.MaintenanceMode
	statusClusterSpec.Data.MaintenanceModeExpiry = actualCluster.ClusterSpec.Data.MaintenanceModeExpiry

	// These pointer fields are server-stamped and echoed by GetCluster
	// but not ExportInstance. Use GetCluster values for comparison so
	// late-initialized defaults settle while user-pinned values still
	// surface as drift when the platform has not applied them.
	overlayClusterGetOnlyData(&driftTarget.ClusterSpec.Data, actualCluster.ClusterSpec.Data)
	overlayClusterGetOnlyData(&statusClusterSpec.Data, actualCluster.ClusterSpec.Data)
	mg.Status.AtProvider.ClusterSpec = statusClusterSpec

	spec := driftSpec()
	desired := mg.Spec.ForProvider
	isUpToDate, err := base.EvaluateDrift(ctx, spec, &desired, &driftTarget, e.Logger, "Cluster")
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	if !isUpToDate {
		if obs, err, ok := e.suppressTerminalWrite(mg); ok {
			return obs, err
		}
	} else {
		e.clearTerminalWrite(mg)
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate,
	}, nil
}

func overlayClusterGetOnlyData(target *generated.ClusterData, actual generated.ClusterData) {
	if target == nil {
		return
	}
	target.MultiClusterK8SDashboardEnabled = actual.MultiClusterK8SDashboardEnabled
	target.AutoscalerConfig = actual.AutoscalerConfig
	target.Compatibility = actual.Compatibility
	target.ArgocdNotificationsSettings = actual.ArgocdNotificationsSettings
	target.DirectClusterSpec = actual.DirectClusterSpec
	target.ServerSideDiffEnabled = actual.ServerSideDiffEnabled
	target.PodInheritMetadata = actual.PodInheritMetadata
	target.DatadogAnnotationsEnabled = actual.DatadogAnnotationsEnabled
	target.EksAddonEnabled = actual.EksAddonEnabled
}

func (e *external) Create(ctx context.Context, mg *v1alpha1.Cluster) (managed.ExternalCreation, error) {
	defer base.PropagateObservedGeneration(mg)

	key, err := clusterTerminalWriteKey(mg)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	req, err := BuildApplyInstanceRequest(mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(fmt.Errorf("%s: %w", errTransformCluster, err)))
	}
	if err := e.Client.ApplyInstance(ctx, req); err != nil {
		return managed.ExternalCreation{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(err))
	}

	// ApplyInstance has now stamped the platform row. Every subsequent
	// failure path must roll the row back so a retried Create starts
	// from a clean slate and a user-initiated kubectl delete (which
	// short-circuits when the external-name annotation is empty) cannot
	// orphan the platform row. Mirrors the cleanup pattern used by the
	// reference Akuity client when applyInstance, reconciliation, or
	// kubeconfig install fails on a fresh resource.
	if err := e.syncMaintenanceMode(ctx, mg); err != nil {
		e.rollbackCreatedCluster(ctx, mg, "syncMaintenanceMode")
		return managed.ExternalCreation{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(err))
	}
	e.ClearTerminalWrite(key)

	// One-time apply: manifests are installed on the managed cluster at
	// Create only. Update intentionally does not re-apply them.
	// Server-pushed agent upgrades require a spec change or recreate to
	// land on the managed cluster. A failure here also records a
	// terminal write so the next Observe short-circuits via the
	// suppress site rather than hot-looping ApplyInstance + rollback
	// every poll. The terminal-write key is keyed off ForProvider, so a
	// spec edit (the only way the user can fix a bad kubeconfig source)
	// rotates the key and lets Create run again.
	if mg.Spec.ForProvider.EnableInClusterKubeConfig || mg.Spec.ForProvider.KubeConfigSecretRef.Name != "" {
		e.Logger.Debug("Retrieving cluster manifests....")
		clusterManifests, err := e.Client.GetClusterManifests(ctx, mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider.Name)
		if err != nil {
			e.rollbackCreatedCluster(ctx, mg, "get-manifests")
			return managed.ExternalCreation{}, e.RecordTerminalWrite(key, reason.AsTerminal(fmt.Errorf("could not get cluster manifests to apply: %w", err)))
		}

		e.Logger.Debug("Applying cluster manifests",
			"clusterName", mg.Name,
			"instanceID", mg.Spec.ForProvider.InstanceID,
		)
		e.Logger.Debug(clusterManifests)
		if err := e.applyClusterManifests(ctx, *mg, clusterManifests, false); err != nil {
			e.rollbackCreatedCluster(ctx, mg, "apply-manifests")
			return managed.ExternalCreation{}, e.RecordTerminalWrite(key, reason.AsTerminal(fmt.Errorf("could not apply cluster manifests: %w", err)))
		}
	}
	meta.SetExternalName(mg, mg.Spec.ForProvider.Name)

	return managed.ExternalCreation{}, nil
}

// rollbackCreatedCluster removes the just-stamped platform row (and
// optionally the partially-installed manifests) when a Create-path step
// fails after ApplyInstance succeeded. Errors during rollback are
// logged at info level rather than returned: the caller's original
// failure is the user-visible error, and a rollback failure should not
// mask it. The platform row is already orphaned at that point, so the
// info log gives operators the breadcrumb without a second error
// surface.
func (e *external) rollbackCreatedCluster(ctx context.Context, mg *v1alpha1.Cluster, stage string) {
	if mg.Spec.ForProvider.RemoveAgentResourcesOnDestroy &&
		(mg.Spec.ForProvider.EnableInClusterKubeConfig || mg.Spec.ForProvider.KubeConfigSecretRef.Name != "") {
		manifests, err := e.Client.GetClusterManifests(ctx, mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider.Name)
		if err == nil {
			if applyErr := e.applyClusterManifests(ctx, *mg, manifests, true); applyErr != nil {
				e.Logger.Info("rollback could not strip partially-installed cluster manifests",
					"stage", stage,
					"error", applyErr,
					"instanceID", mg.Spec.ForProvider.InstanceID,
					"clusterName", mg.Spec.ForProvider.Name,
				)
			}
		}
	}
	if err := e.Client.DeleteCluster(ctx, mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider.Name); err != nil {
		e.Logger.Info("rollback after create failure left platform row stamped",
			"stage", stage,
			"error", err,
			"instanceID", mg.Spec.ForProvider.InstanceID,
			"clusterName", mg.Spec.ForProvider.Name,
		)
	}
}

func (e *external) Update(ctx context.Context, mg *v1alpha1.Cluster) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)

	key, err := clusterTerminalWriteKey(mg)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	req, err := BuildApplyInstanceRequest(mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(fmt.Errorf("%s: %w", errTransformCluster, err)))
	}
	if err := e.Client.ApplyInstance(ctx, req); err != nil {
		return managed.ExternalUpdate{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(err))
	}
	if err := e.syncMaintenanceMode(ctx, mg); err != nil {
		return managed.ExternalUpdate{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(err))
	}
	e.ClearTerminalWrite(key)
	return managed.ExternalUpdate{}, nil
}

// syncMaintenanceMode pushes data.maintenanceMode and data.maintenanceModeExpiry
// through the dedicated set-maintenance-mode endpoint when the user has
// configured either field. ApplyInstance silently drops both fields, so
// without this separate RPC the user-set state never reaches the
// platform and the drift comparator would keep scheduling Apply. Before
// this RPC, the loop produced roughly 4 wasted writes in 2 minutes.
// This uses the platform's dedicated maintenance-mode mutation path.
//
// When neither field is configured (both *bool nil and *string nil) the
// call is skipped, avoiding an implicit clear of a value the user did
// not ask this resource to control.
func (e *external) syncMaintenanceMode(ctx context.Context, mg *v1alpha1.Cluster) error {
	data := mg.Spec.ForProvider.ClusterSpec.Data
	if data.MaintenanceMode == nil && data.MaintenanceModeExpiry == nil {
		return nil
	}
	mode := false
	if data.MaintenanceMode != nil {
		mode = *data.MaintenanceMode
	}
	var expiry *time.Time
	if data.MaintenanceModeExpiry != nil && *data.MaintenanceModeExpiry != "" {
		t, err := time.Parse(time.RFC3339, *data.MaintenanceModeExpiry)
		if err != nil {
			return reason.AsTerminal(fmt.Errorf("could not parse spec.forProvider.data.maintenanceModeExpiry as RFC3339: %w", err))
		}
		expiry = &t
	}
	return e.Client.SetClusterMaintenanceMode(ctx, mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider.Name, mode, expiry)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha1.Cluster) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)
	e.ClearTerminalWriteResource(mg, v1alpha1.ClusterGroupVersionKind)

	externalName := meta.GetExternalName(mg)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	if mg.Spec.ForProvider.RemoveAgentResourcesOnDestroy &&
		(mg.Spec.ForProvider.EnableInClusterKubeConfig || mg.Spec.ForProvider.KubeConfigSecretRef.Name != "") {
		clusterManifests, err := e.Client.GetClusterManifests(ctx, mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider.Name)
		if err != nil {
			return managed.ExternalDelete{}, fmt.Errorf("could not get cluster manifests to delete: %w", err)
		}

		if err := e.applyClusterManifests(ctx, *mg, clusterManifests, true); err != nil {
			return managed.ExternalDelete{}, fmt.Errorf("could not delete cluster manifests: %w", err)
		}
	}

	if err := e.Client.DeleteCluster(ctx, mg.Spec.ForProvider.InstanceID, externalName); err != nil {
		return managed.ExternalDelete{}, fmt.Errorf("could not delete cluster: %w", err)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

func (e *external) suppressTerminalWrite(mg *v1alpha1.Cluster) (managed.ExternalObservation, error, bool) {
	if e.TerminalWrites == nil {
		return managed.ExternalObservation{}, nil, false
	}
	key, err := clusterTerminalWriteKey(mg)
	if err != nil {
		return e.SkipTerminalWriteGuard(err)
	}
	return e.SuppressTerminalWrite(mg, key)
}

func clusterTerminalWriteKey(mg *v1alpha1.Cluster) (base.TerminalWriteKey, error) {
	return base.NewTerminalWriteKey(mg, v1alpha1.ClusterGroupVersionKind, mg.Spec.ForProvider)
}

func (e *external) clearTerminalWrite(mg *v1alpha1.Cluster) {
	if !e.HasTerminalWriteResource(mg, v1alpha1.ClusterGroupVersionKind) {
		return
	}
	key, err := clusterTerminalWriteKey(mg)
	if err != nil {
		e.LogTerminalWriteGuardSkipped(err)
		return
	}
	e.ClearTerminalWrite(key)
}

func lateInitializeCluster(in *v1alpha1.ClusterParameters, actual v1alpha1.ClusterParameters) {
	in.Namespace = pointer.LateInitialize(in.Namespace, actual.Namespace)
	in.ClusterSpec.Data.AutoUpgradeDisabled = pointer.LateInitialize(in.ClusterSpec.Data.AutoUpgradeDisabled, actual.ClusterSpec.Data.AutoUpgradeDisabled)
	in.ClusterSpec.Data.AppReplication = pointer.LateInitialize(in.ClusterSpec.Data.AppReplication, actual.ClusterSpec.Data.AppReplication)
	in.ClusterSpec.Data.TargetVersion = pointer.LateInitialize(in.ClusterSpec.Data.TargetVersion, actual.ClusterSpec.Data.TargetVersion)
	in.ClusterSpec.Data.RedisTunneling = pointer.LateInitialize(in.ClusterSpec.Data.RedisTunneling, actual.ClusterSpec.Data.RedisTunneling)
	in.ClusterSpec.Data.Size = pointer.LateInitialize(in.ClusterSpec.Data.Size, actual.ClusterSpec.Data.Size)
	in.ClusterSpec.Data.Kustomization = pointer.LateInitialize(in.ClusterSpec.Data.Kustomization, actual.ClusterSpec.Data.Kustomization)
	in.ClusterSpec.Data.MultiClusterK8SDashboardEnabled = pointer.LateInitialize(in.ClusterSpec.Data.MultiClusterK8SDashboardEnabled, actual.ClusterSpec.Data.MultiClusterK8SDashboardEnabled)
	in.ClusterSpec.Data.AutoscalerConfig = pointer.LateInitialize(in.ClusterSpec.Data.AutoscalerConfig, actual.ClusterSpec.Data.AutoscalerConfig)
	in.ClusterSpec.Data.PodInheritMetadata = pointer.LateInitialize(in.ClusterSpec.Data.PodInheritMetadata, actual.ClusterSpec.Data.PodInheritMetadata)
}

// exportedClusterSpec returns the canonical ClusterParameters for
// clusterName built from ExportInstanceByID's response. The returned
// `found` flag is false when Export succeeds but the named cluster is
// absent from the response (e.g. server mid-provisioning or a
// concurrent sibling deletion); caller falls back to GetCluster-based
// drift in that case. Transport failures are logged and treated as not
// found so observation can fall back to Get; errors are decode/schema
// failures in the Export payload.
func (e *external) exportedClusterSpec(ctx context.Context, instanceID, clusterName string, desired v1alpha1.ClusterParameters) (v1alpha1.ClusterParameters, bool, error) {
	exp, err := e.Client.ExportInstanceByID(ctx, instanceID)
	if err != nil {
		e.Logger.Debug("ExportInstanceByID failed; falling back to GetCluster for drift", "err", err)
		return v1alpha1.ClusterParameters{}, false, nil
	}
	for _, entry := range exp.GetClusters() {
		if entry == nil {
			continue
		}
		wire := &akuitytypes.Cluster{}
		raw, merr := entry.MarshalJSON()
		if merr != nil {
			return v1alpha1.ClusterParameters{}, false, fmt.Errorf("encode exported cluster: %w", merr)
		}
		if uerr := json.Unmarshal(raw, wire); uerr != nil {
			return v1alpha1.ClusterParameters{}, false, fmt.Errorf("decode exported cluster: %w", uerr)
		}
		if wire.GetName() != clusterName {
			continue
		}
		return wireToSpec(instanceID, desired, wire), true, nil
	}
	return v1alpha1.ClusterParameters{}, false, nil
}

func (e *external) getInstanceID(ctx context.Context, instanceID string, instanceRef *v1alpha1.LocalReference) (string, error) {
	if instanceID != "" {
		return instanceID, nil
	}

	if instanceRef == nil || instanceRef.Name == "" {
		return "", fmt.Errorf("one of InstanceID or InstanceRef must be provided")
	}

	instance := &v1alpha1.Instance{}
	if err := e.Kube.Get(ctx, k8stypes.NamespacedName{Name: instanceRef.Name}, instance); err != nil {
		return "", fmt.Errorf("could not look up instance with instanceRef %s: %w", instanceRef.Name, err)
	}

	akuityInstance, err := e.Client.GetInstance(ctx, instance.Spec.ForProvider.Name)
	if err != nil {
		return "", fmt.Errorf("could not look up instance with instanceRef %s: %w", instanceRef.Name, err)
	}

	return akuityInstance.GetId(), nil
}

// targetKubeConfig returns the TargetKubeConfig for mg, used by the
// shared manifest-apply helper.
func targetKubeConfig(mg v1alpha1.Cluster) kube.TargetKubeConfig {
	return kube.TargetKubeConfig{
		EnableInCluster: mg.Spec.ForProvider.EnableInClusterKubeConfig,
		SecretName:      mg.Spec.ForProvider.KubeConfigSecretRef.Name,
		SecretNamespace: mg.Spec.ForProvider.KubeConfigSecretRef.Namespace,
	}
}

func (e *external) applyClusterManifests(ctx context.Context, mg v1alpha1.Cluster, clusterManifests string, delete bool) error {
	return kube.ApplyManifestsToTarget(ctx, e.Kube, e.Logger, targetKubeConfig(mg), clusterManifests, delete)
}

// driftSpec is the Cluster drift-detection recipe. Normalize bridges
// the lateInit-vs-Export shape gap: lateInitializeCluster adopts
// server defaults from GetCluster, while ExportInstanceByID omits some
// of those defaults. Without normalization, desired and observed would
// disagree on fields neither the user nor the server treats as drift.
func driftSpec() base.DriftSpec[v1alpha1.ClusterParameters] {
	return base.DriftSpec[v1alpha1.ClusterParameters]{
		Normalize: func(desired, observed *v1alpha1.ClusterParameters) {
			if desired == nil || observed == nil {
				return
			}

			normalizePtrField(
				&desired.ClusterSpec.Data.MultiClusterK8SDashboardEnabled,
				&observed.ClusterSpec.Data.MultiClusterK8SDashboardEnabled,
			)
			normalizePtrField(
				&desired.ClusterSpec.Data.AutoscalerConfig,
				&observed.ClusterSpec.Data.AutoscalerConfig,
			)
			normalizePtrField(
				&desired.ClusterSpec.Data.Compatibility,
				&observed.ClusterSpec.Data.Compatibility,
			)
			normalizePtrField(
				&desired.ClusterSpec.Data.ArgocdNotificationsSettings,
				&observed.ClusterSpec.Data.ArgocdNotificationsSettings,
			)
			normalizePtrField(
				&desired.ClusterSpec.Data.DirectClusterSpec,
				&observed.ClusterSpec.Data.DirectClusterSpec,
			)
			normalizePtrField(
				&desired.ClusterSpec.Data.ServerSideDiffEnabled,
				&observed.ClusterSpec.Data.ServerSideDiffEnabled,
			)
			normalizePtrField(
				&desired.ClusterSpec.Data.DatadogAnnotationsEnabled,
				&observed.ClusterSpec.Data.DatadogAnnotationsEnabled,
			)
			normalizePtrField(
				&desired.ClusterSpec.Data.EksAddonEnabled,
				&observed.ClusterSpec.Data.EksAddonEnabled,
			)
			normalizePtrField(
				&desired.ClusterSpec.Data.PodInheritMetadata,
				&observed.ClusterSpec.Data.PodInheritMetadata,
			)

			// Kustomization is a YAML string round-trip. GetCluster may
			// return "{}\n" while Export returns the canonical
			// "apiVersion/kind" scaffold; both represent an empty
			// Kustomization, so adopt observed when both sides parse empty.
			// Also flatten trailing-newline-only differences: the platform
			// stores the string verbatim, so "value" and "value\n" would
			// otherwise fire ApplyCluster every poll while the server
			// short-circuits the write.
			if kustomizationEquivalent(desired.ClusterSpec.Data.Kustomization, observed.ClusterSpec.Data.Kustomization) {
				desired.ClusterSpec.Data.Kustomization = observed.ClusterSpec.Data.Kustomization
			}
		},
	}
}

// normalizePtrField adopts the observed pointer value onto desired
// when desired is unset and observed carries a server-stamped value.
// This is the safe direction of the lateInit-vs-Export bridge: the
// user did not pin anything, so inheriting the platform default avoids
// per-poll drift on values the user does not control.
//
// The reverse direction (desired set, observed nil) is intentionally
// not handled here. Collapsing desired to nil would drop user-pinned
// values whenever the platform has not observed them yet. Real drift
// must surface so Apply runs and either persists the value or returns
// the platform's rejection.
func normalizePtrField[T any](desired, observed **T) {
	if *desired == nil && *observed != nil {
		*desired = *observed
	}
}

// kustomizationEmptyEquivalent returns true when both Kustomization
// strings parse to a structurally-empty Kustomization manifest: that is,
// only the scaffold keys (apiVersion, kind) plus zero or more empty
// arrays/objects, no resources/patches/generators/etc. that would
// represent real user intent. Comparison is lenient: if either side
// fails to parse, fall back to raw byte equality so malformed user
// input surfaces as drift.
func kustomizationEmptyEquivalent(a, b string) bool {
	if a == b {
		return true
	}
	aEmpty, aErr := isEmptyKustomization(a)
	bEmpty, bErr := isEmptyKustomization(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return aEmpty && bEmpty
}

func kustomizationEquivalent(a, b string) bool {
	if kustomizationEmptyEquivalent(a, b) {
		return true
	}
	if strings.TrimRight(a, "\n") == strings.TrimRight(b, "\n") {
		return true
	}
	aRaw, aErr := clusterKustomizationRaw(a)
	bRaw, bErr := clusterKustomizationRaw(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return bytes.Equal(aRaw.Raw, bRaw.Raw)
}

// isEmptyKustomization decides whether a Kustomization YAML string
// represents an "empty" manifest: it contains nothing the server
// would treat as actual configuration. The empty cases are:
//   - "" or whitespace-only
//   - "{}" / "{}\n" / "null"
//   - Only apiVersion + kind keys present, no resources/patches/etc.
//
// Any other key under the root object means the user has put real
// content into Kustomization and should not collapse drift.
func isEmptyKustomization(s string) (bool, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return true, nil
	}
	raw, err := sigsyaml.YAMLToJSON([]byte(s))
	if err != nil {
		return false, err
	}
	// Decode into any first; only inspect keys when the top-level is a
	// JSON object. Non-object top-level (null, array, scalar) is treated
	// as structurally empty since none of those carry Kustomization
	// configuration the server would act on.
	var top any
	if err := json.Unmarshal(raw, &top); err != nil {
		return false, err
	}
	v, ok := top.(map[string]any)
	if !ok {
		return true, nil
	}
	for k := range v {
		if k == "apiVersion" || k == "kind" {
			continue
		}
		// Any other key represents user-set Kustomization content.
		return false, nil
	}
	return true, nil
}
