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
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
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

	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// GetCluster stays as the source of truth for observation data
	// (HealthStatus / ReconciliationStatus / AgentState / agent size /
	// target version etc.) — none of that round-trips through Export.
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

	// Drift compares against ExportInstanceByID's round-trippable spec
	// (same structural shape ApplyInstance{Clusters:[one]} sends).
	// Pattern-consistent with Instance/KargoInstance: read via Export,
	// write via Apply. GetCluster above is still load-bearing for
	// observation fields (health/reconciliation/agent state) that
	// Export does not return. If Export succeeds but the cluster entry
	// is missing from its Clusters list we fall back to the
	// GetCluster-derived spec: GetCluster saw it, so it does exist —
	// Export was just lagging.
	driftTarget, found, err := e.exportedClusterSpec(ctx, instanceID, meta.GetExternalName(mg), mg.Spec.ForProvider)
	if err != nil {
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}
	if !found {
		driftTarget = actualCluster
	}

	spec := driftSpec()
	desired := mg.Spec.ForProvider
	isUpToDate, err := base.EvaluateDrift(ctx, spec, &desired, &driftTarget, e.Logger, "Cluster")
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mg *v1alpha1.Cluster) (managed.ExternalCreation, error) {
	defer base.PropagateObservedGeneration(mg)

	req, err := BuildApplyInstanceRequest(mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errTransformCluster)
	}
	if err := e.Client.ApplyInstance(ctx, req); err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("could not create cluster: %w", err)
	}

	if mg.Spec.ForProvider.EnableInClusterKubeConfig || mg.Spec.ForProvider.KubeConfigSecretRef.Name != "" {
		e.Logger.Debug("Retrieving cluster manifests....")
		clusterManifests, err := e.Client.GetClusterManifests(ctx, mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider.Name)
		if err != nil {
			return managed.ExternalCreation{}, fmt.Errorf("could not get cluster manifests to apply: %w", err)
		}

		e.Logger.Debug("Applying cluster manifests",
			"clusterName", mg.Name,
			"instanceID", mg.Spec.ForProvider.InstanceID,
		)
		e.Logger.Debug(clusterManifests)
		if err := e.applyClusterManifests(ctx, *mg, clusterManifests, false); err != nil {
			return managed.ExternalCreation{}, fmt.Errorf("could not apply cluster manifests: %w", err)
		}
	}
	meta.SetExternalName(mg, mg.Spec.ForProvider.Name)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha1.Cluster) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)

	req, err := BuildApplyInstanceRequest(mg.Spec.ForProvider.InstanceID, mg.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errTransformCluster)
	}
	return managed.ExternalUpdate{}, e.Client.ApplyInstance(ctx, req)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha1.Cluster) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)

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
}

// exportedClusterSpec returns the canonical ClusterParameters for
// clusterName built from ExportInstanceByID's response. The returned
// `found` flag is false when Export succeeds but the named cluster is
// absent from the response (e.g. server mid-provisioning or a
// concurrent sibling deletion); caller falls back to GetCluster-based
// drift in that case. Errors are transient Export failures only.
func (e *external) exportedClusterSpec(ctx context.Context, instanceID, clusterName string, desired v1alpha1.ClusterParameters) (v1alpha1.ClusterParameters, bool, error) {
	exp, err := e.Client.ExportInstanceByID(ctx, instanceID)
	if err != nil {
		return v1alpha1.ClusterParameters{}, false, err
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

func (e *external) GetClusterKubeClientRestConfig(ctx context.Context, mg v1alpha1.Cluster) (*rest.Config, error) {
	var restConfig *rest.Config
	var err error
	if mg.Spec.ForProvider.EnableInClusterKubeConfig {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return restConfig, fmt.Errorf("could not build in cluster kube config: %w", err)
		}
	} else {
		secretName := mg.Spec.ForProvider.KubeConfigSecretRef.Name
		secretNamespace := mg.Spec.ForProvider.KubeConfigSecretRef.Namespace
		secret := &corev1.Secret{}
		if err := e.Kube.Get(ctx, k8stypes.NamespacedName{Name: secretName, Namespace: mg.Spec.ForProvider.KubeConfigSecretRef.Namespace}, secret); err != nil {
			return restConfig, fmt.Errorf("could not get secret %s in namespace %s containing cluster kubeconfig: %w", secretName, secretNamespace, err)
		}

		kubeConfig, ok := secret.Data["kubeconfig"]
		if !ok {
			return restConfig, fmt.Errorf("could not get cluster kubeconfig from secret %s in namespace %s", secretName, secretNamespace)
		}

		restConfig, err = clientcmd.RESTConfigFromKubeConfig(kubeConfig)
		if err != nil {
			return restConfig, fmt.Errorf("could not get rest config from kubeconfig in secret %s in namespace %s: %w", secretName, secretNamespace, err)
		}
	}

	return restConfig, nil
}

func (e *external) applyClusterManifests(ctx context.Context, mg v1alpha1.Cluster, clusterManifests string, delete bool) error {
	restConfig, err := e.GetClusterKubeClientRestConfig(ctx, mg)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error creating dynamic client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error creating typed client: %w", err)
	}

	applyClient, err := kube.NewApplyClient(dynamicClient, clientset, e.Logger)
	if err != nil {
		return fmt.Errorf("error creating apply client: %w", err)
	}

	return applyClient.ApplyManifests(ctx, clusterManifests, delete)
}

// driftSpec is the Cluster drift-detection recipe. Normalize adopts
// server-defaulted fields that the API unconditionally populates on
// every Apply response, which would otherwise drift-flap every poll:
// desired=nil (or structurally-empty Kustomization) vs observed=populated
// fires an Apply that the server's Equals() short-circuits (DB-gen stays
// frozen) but still burns client throughput. EquateEmpty is contributed
// by the shared DriftSpec baseline.
//
// Covers §6 rows 9 + 10 (naive ClusterSpec.Equals, zero-struct
// Compatibility/ArgocdNotificationsSettings echo). Structurally-equal
// Kustomization handling collapses any YAML representation ("{}\n" vs
// the full "apiVersion: ... kind: Kustomization\n" scaffold) to the
// server's form so the two compare as equal.
func driftSpec() base.DriftSpec[v1alpha1.ClusterParameters] {
	return base.DriftSpec[v1alpha1.ClusterParameters]{
		Normalize: func(desired, observed *v1alpha1.ClusterParameters) {
			if desired == nil || observed == nil {
				return
			}
			if desired.ClusterSpec.Data.MultiClusterK8SDashboardEnabled == nil {
				desired.ClusterSpec.Data.MultiClusterK8SDashboardEnabled =
					observed.ClusterSpec.Data.MultiClusterK8SDashboardEnabled
			}

			// Server echoes a populated AutoscalerConfig (with
			// ApplicationController + RepoServer child structs carrying
			// memory/cpu defaults) even when the CR omits the field.
			if desired.ClusterSpec.Data.AutoscalerConfig == nil {
				desired.ClusterSpec.Data.AutoscalerConfig =
					observed.ClusterSpec.Data.AutoscalerConfig
			}

			// Compatibility {Ipv6Only: false} and
			// ArgocdNotificationsSettings {InClusterSettings: false} are
			// always returned as non-nil zero structs from the server.
			if desired.ClusterSpec.Data.Compatibility == nil {
				desired.ClusterSpec.Data.Compatibility =
					observed.ClusterSpec.Data.Compatibility
			}
			if desired.ClusterSpec.Data.ArgocdNotificationsSettings == nil {
				desired.ClusterSpec.Data.ArgocdNotificationsSettings =
					observed.ClusterSpec.Data.ArgocdNotificationsSettings
			}

			// Kustomization is a YAML string. The server accepts any
			// YAML-equivalent representation (including the empty "{}")
			// but echoes back the canonical "apiVersion: ...\nkind:
			// Kustomization\n" scaffold. Treat equivalent YAML payloads
			// as equal to avoid drift-flap on initial bring-up.
			if equalKustomizationYAML(desired.ClusterSpec.Data.Kustomization, observed.ClusterSpec.Data.Kustomization) {
				desired.ClusterSpec.Data.Kustomization = observed.ClusterSpec.Data.Kustomization
			}
		},
	}
}

// equalKustomizationYAML returns true when two Kustomization YAML
// strings parse to the same JSON payload. Empty strings and "{}\n" and
// the server-echoed "apiVersion: kustomize.config.k8s.io/v1beta1\nkind:
// Kustomization\n" all parse to the same empty-or-scaffold object from
// the server's perspective — comparing as raw strings would flap every
// poll. Comparison is lenient: if either side fails to parse, fall
// back to raw equality so malformed user input surfaces as drift.
func equalKustomizationYAML(a, b string) bool {
	if a == b {
		return true
	}
	aJSON, aErr := yamlToCanonicalJSON(a)
	bJSON, bErr := yamlToCanonicalJSON(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return aJSON == bJSON
}

func yamlToCanonicalJSON(s string) (string, error) {
	if s == "" {
		return "{}", nil
	}
	raw, err := sigsyaml.YAMLToJSON([]byte(s))
	if err != nil {
		return "", err
	}
	// Re-marshal through encoding/json to drop map-order variation.
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	if v == nil {
		return "{}", nil
	}
	out, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
