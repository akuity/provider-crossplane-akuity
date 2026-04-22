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

// Package cluster is the v1alpha2 Cluster controller. It mirrors the
// v1alpha1 agent-install behaviour: when a kubeconfig source is
// configured on the spec (kubeconfigSecretRef or
// enableInClusterKubeconfig) the controller fetches the
// Akuity-generated install manifests and server-side applies them to
// the managed cluster via internal/clients/kube. Re-apply is driven by
// the AgentManifestsHash field on AtProvider; on deletion, when
// removeAgentResourcesOnDestroy is set, the same apply path runs
// in delete mode before the platform-side DeleteCluster call.
//
// If no kubeconfig source is configured the controller manages only
// the Akuity platform record and never touches the managed cluster.
package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	reconv1 "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/event"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	apisv1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/kube"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert/glue"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// kubeconfigSecretKey is the data key on the referenced Secret that
// must hold the kubeconfig. Matches the v1alpha1 contract.
const kubeconfigSecretKey = "kubeconfig"

// clusterDriftOpts filters fields that are write-only on the current
// api-client-go proto — i.e. the controller can send them on apply but
// cannot read them back, so including them in cmp.Equal would drift
// flap forever. Empty post the v0.29.1 bump which added proto read
// support for PodInheritMetadata; retained as an extension point.
//
// KubeConfigSecretRef / EnableInClusterKubeConfig /
// RemoveAgentResourcesOnDestroy are controller-side knobs that the
// Akuity API never reports back, so we exclude them from drift
// comparison too.
var clusterDriftOpts = []cmp.Option{
	cmpopts.IgnoreFields(v1alpha2.ClusterParameters{},
		"KubeConfigSecretRef",
		"EnableInClusterKubeConfig",
		"RemoveAgentResourcesOnDestroy",
	),
}

// Setup registers the v1alpha2 Cluster controller.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha2.ClusterGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

	conn := &base.Connector[*v1alpha2.Cluster]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha2.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha2.Cluster] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.ClusterGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha2.Cluster](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.Cluster{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha2.Cluster) (managed.ExternalObservation, error) {
	defer base.PropagateObservedGeneration(mg)
	instanceID, err := e.resolveInstanceID(ctx, mg)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	mg.Spec.ForProvider.InstanceID = instanceID

	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ac, obs, done, err := e.fetchCluster(ctx, mg, instanceID)
	if done {
		return obs, err
	}

	// Project the observed cluster into the v1alpha2 shape and the
	// AtProvider status block regardless of reconciliation progress.
	//
	// AgentManifestsHash is controller-managed state (the last hash we
	// successfully installed onto the managed cluster) and is not
	// reflected by the Akuity API — preserve it across the
	// apiToObservation overwrite, same pattern the Instance/KargoInstance
	// controllers use for SecretHash.
	actual := apiToSpec(instanceID, mg.Spec.ForProvider, ac)
	prevAgentHash := mg.Status.AtProvider.AgentManifestsHash
	mg.Status.AtProvider = apiToObservation(ac)
	mg.Status.AtProvider.AgentManifestsHash = prevAgentHash

	// Surface health conditions. Reconciliation-in-progress clusters
	// report Unavailable via their HealthStatus; terminal reconciliation
	// produces the Available signal.
	if mg.Status.AtProvider.HealthStatus.Code != 1 {
		mg.SetConditions(xpv1.Unavailable())
	} else {
		mg.SetConditions(xpv1.Available())
	}

	// Always compare spec so a matching desired/observed pair does not
	// trigger Update() on every poll while reconciliation is still
	// pending. Returning ResourceUpToDate=false during provisioning
	// caused a hot-loop of ApplyCluster calls on the Akuity API.
	upToDate := cmp.Equal(mg.Spec.ForProvider, actual, clusterDriftOpts...)
	if !upToDate {
		e.Logger.Debug("Cluster drift detected", "diff", cmp.Diff(mg.Spec.ForProvider, actual, clusterDriftOpts...))
	}

	observation := managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}

	drifted, err := e.agentManifestsDrifted(ctx, mg, ac, instanceID)
	if err != nil {
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}
	if drifted {
		observation.ResourceUpToDate = false
	}

	return observation, nil
}

// agentManifestsDrifted reports whether the Akuity-generated install
// manifests differ from the last bundle the controller successfully
// applied to the managed cluster (tracked via AgentManifestsHash on
// AtProvider). Returns false when no kubeconfig source is configured
// or reconciliation on the platform side is not yet terminal — in the
// latter case a manifests fetch would return partial content.
func (e *external) agentManifestsDrifted(ctx context.Context, mg *v1alpha2.Cluster, ac *argocdv1.Cluster, instanceID string) (bool, error) {
	if !hasKubeConfigSource(&mg.Spec.ForProvider) {
		return false, nil
	}
	reconCode := ac.GetReconciliationStatus().GetCode()
	if reconCode != reconv1.StatusCode_STATUS_CODE_SUCCESSFUL && reconCode != reconv1.StatusCode_STATUS_CODE_FAILED {
		return false, nil
	}
	manifests, err := e.Client.GetClusterManifestsOnce(ctx, instanceID, ac.GetId())
	if err != nil {
		return false, err
	}
	if manifests == "" {
		return false, nil
	}
	desired := hashManifests(manifests)
	if desired == mg.Status.AtProvider.AgentManifestsHash {
		return false, nil
	}
	e.Logger.Debug("Agent manifests drift detected",
		"previous", mg.Status.AtProvider.AgentManifestsHash,
		"current", desired)
	return true, nil
}

// fetchCluster wraps e.Client.GetCluster and folds the three
// transient/error classifications into an ExternalObservation ready to
// return. The `done` return is true when the caller (Observe) should
// short-circuit with the returned observation/err; on done=false the
// cluster was fetched successfully and the caller continues.
func (e *external) fetchCluster(ctx context.Context, mg *v1alpha2.Cluster, instanceID string) (*argocdv1.Cluster, managed.ExternalObservation, bool, error) {
	ac, err := e.Client.GetCluster(ctx, instanceID, meta.GetExternalName(mg))
	if err == nil {
		return ac, managed.ExternalObservation{}, false, nil
	}
	if reason.IsNotFound(err) {
		return nil, managed.ExternalObservation{ResourceExists: false}, true, nil
	}
	if reason.IsProvisioningWait(err) {
		// Transient wait-state — surface as Unavailable rather than
		// escalating to ReconcileError. UpToDate=true avoids the
		// provisioning hot-loop.
		mg.SetConditions(xpv1.Unavailable())
		return nil, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, true, nil
	}
	mg.SetConditions(xpv1.ReconcileError(err))
	return nil, managed.ExternalObservation{}, true, err
}

func (e *external) Create(ctx context.Context, mg *v1alpha2.Cluster) (managed.ExternalCreation, error) {
	defer base.PropagateObservedGeneration(mg)
	if err := e.apply(ctx, mg); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.Spec.ForProvider.Name)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha2.Cluster) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)
	return managed.ExternalUpdate{}, e.apply(ctx, mg)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha2.Cluster) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)
	name := meta.GetExternalName(mg)
	if name == "" {
		return managed.ExternalDelete{}, nil
	}

	// Target-cluster teardown must happen before the platform-side
	// DeleteCluster call: once the platform record is gone the
	// GetClusterManifests lookup would fail and we would lose the
	// blueprint for what to tear down.
	if mg.Spec.ForProvider.RemoveAgentResourcesOnDestroy && hasKubeConfigSource(&mg.Spec.ForProvider) {
		if err := e.removeAgentFromTarget(ctx, mg); err != nil {
			return managed.ExternalDelete{}, err
		}
	}

	if err := e.Client.DeleteCluster(ctx, mg.Spec.ForProvider.InstanceID, name); err != nil {
		return managed.ExternalDelete{}, fmt.Errorf("could not delete cluster: %w", err)
	}
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error { return nil }

// apply is shared by Create and Update. The Akuity ApplyCluster call
// is idempotent; the optional target-side agent install is best-effort
// in the sense that if manifests are not yet ready (platform still
// reconciling) we return nil and let the next Observe/Update round-trip
// drive it to completion.
func (e *external) apply(ctx context.Context, mg *v1alpha2.Cluster) error {
	if err := glue.ValidateKustomizationYAML(mg.Spec.ForProvider.Data.Kustomization); err != nil {
		return fmt.Errorf("spec.forProvider.data.kustomization: %w", err)
	}

	instanceID, err := e.resolveInstanceID(ctx, mg)
	if err != nil {
		return err
	}
	mg.Spec.ForProvider.InstanceID = instanceID

	wire := specToAPI(mg.Spec.ForProvider)
	if err := e.Client.ApplyCluster(ctx, instanceID, wire); err != nil {
		return fmt.Errorf("could not apply cluster: %w", err)
	}

	if hasKubeConfigSource(&mg.Spec.ForProvider) {
		if err := e.installAgentOnTarget(ctx, mg, instanceID); err != nil {
			return err
		}
	}
	return nil
}

// installAgentOnTarget fetches the Akuity-generated install manifests
// and server-side applies them to the managed cluster. No-op when
// reconciliation is not yet terminal (manifest body would be partial);
// the next Observe poll will flag drift and we retry here on Update.
func (e *external) installAgentOnTarget(ctx context.Context, mg *v1alpha2.Cluster, instanceID string) error {
	ac, err := e.Client.GetCluster(ctx, instanceID, meta.GetExternalName(mg))
	if err != nil {
		if reason.IsNotFound(err) || reason.IsProvisioningWait(err) {
			// Platform side has not caught up with our ApplyCluster yet.
			// Next Observe will retry; no action here.
			return nil
		}
		return fmt.Errorf("could not look up cluster to install agent: %w", err)
	}
	reconCode := ac.GetReconciliationStatus().GetCode()
	if reconCode != reconv1.StatusCode_STATUS_CODE_SUCCESSFUL && reconCode != reconv1.StatusCode_STATUS_CODE_FAILED {
		return nil
	}

	manifests, err := e.Client.GetClusterManifestsOnce(ctx, instanceID, ac.GetId())
	if err != nil {
		return fmt.Errorf("could not get cluster manifests: %w", err)
	}
	if manifests == "" {
		return nil
	}

	cfg, err := e.getTargetRestConfig(ctx, mg)
	if err != nil {
		return err
	}
	if err := e.applyAgentManifests(ctx, cfg, manifests, false); err != nil {
		return fmt.Errorf("could not apply agent manifests to managed cluster: %w", err)
	}
	mg.Status.AtProvider.AgentManifestsHash = hashManifests(manifests)
	return nil
}

// removeAgentFromTarget fetches the latest install manifests and
// deletes them from the managed cluster. Intended for the Delete path
// when removeAgentResourcesOnDestroy is set.
func (e *external) removeAgentFromTarget(ctx context.Context, mg *v1alpha2.Cluster) error {
	instanceID := mg.Spec.ForProvider.InstanceID
	if instanceID == "" {
		resolved, err := e.resolveInstanceID(ctx, mg)
		if err != nil {
			return err
		}
		instanceID = resolved
	}

	ac, err := e.Client.GetCluster(ctx, instanceID, meta.GetExternalName(mg))
	if err != nil {
		if reason.IsNotFound(err) {
			// Platform-side record already gone — nothing to tear down.
			return nil
		}
		return fmt.Errorf("could not look up cluster to remove agent: %w", err)
	}

	manifests, err := e.Client.GetClusterManifestsOnce(ctx, instanceID, ac.GetId())
	if err != nil {
		return fmt.Errorf("could not get cluster manifests for cleanup: %w", err)
	}
	if manifests == "" {
		return nil
	}

	cfg, err := e.getTargetRestConfig(ctx, mg)
	if err != nil {
		return err
	}
	if err := e.applyAgentManifests(ctx, cfg, manifests, true); err != nil {
		return fmt.Errorf("could not remove agent manifests from managed cluster: %w", err)
	}
	return nil
}

// getTargetRestConfig returns the REST config the controller uses to
// talk to the managed cluster. Exactly one of KubeConfigSecretRef or
// EnableInClusterKubeConfig must be set; mutual exclusion is already
// enforced by CEL on the CRD.
func (e *external) getTargetRestConfig(ctx context.Context, mg *v1alpha2.Cluster) (*rest.Config, error) {
	p := &mg.Spec.ForProvider
	if p.EnableInClusterKubeConfig {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("build in-cluster kubeconfig: %w", err)
		}
		return cfg, nil
	}

	if p.KubeConfigSecretRef == nil || p.KubeConfigSecretRef.Name == "" {
		// Guard against callers invoking this without a configured
		// source; hasKubeConfigSource() is the canonical check.
		return nil, fmt.Errorf("no kubeconfig source configured on spec.forProvider")
	}

	key := k8stypes.NamespacedName{Name: p.KubeConfigSecretRef.Name, Namespace: mg.GetNamespace()}
	s := &corev1.Secret{}
	if err := e.Kube.Get(ctx, key, s); err != nil {
		return nil, fmt.Errorf("get kubeconfig Secret %s: %w", key, err)
	}
	raw, ok := s.Data[kubeconfigSecretKey]
	if !ok {
		return nil, fmt.Errorf("secret %s is missing required data key %q", key, kubeconfigSecretKey)
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig from Secret %s: %w", key, err)
	}
	return cfg, nil
}

// applyAgentManifests wraps internal/clients/kube.NewApplyClient to
// server-side apply (remove=false) or delete (remove=true) the
// multi-document manifest bundle against the managed cluster.
func (e *external) applyAgentManifests(ctx context.Context, cfg *rest.Config, manifests string, remove bool) error {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create typed client: %w", err)
	}
	ac, err := kube.NewApplyClient(dyn, cs, e.Logger)
	if err != nil {
		return fmt.Errorf("create apply client: %w", err)
	}
	return ac.ApplyManifests(ctx, manifests, remove)
}

// hasKubeConfigSource reports whether the user has opted into inline
// agent apply. Mutual exclusion is enforced by CEL on the CRD.
func hasKubeConfigSource(p *v1alpha2.ClusterParameters) bool {
	if p.EnableInClusterKubeConfig {
		return true
	}
	return p.KubeConfigSecretRef != nil && p.KubeConfigSecretRef.Name != ""
}

func hashManifests(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// resolveInstanceID returns the Akuity ArgoCD instance ID either from
// the spec directly (InstanceID) or by resolving InstanceRef. v1alpha2
// restricts InstanceRef resolution to the same namespace as the
// Cluster.
func (e *external) resolveInstanceID(ctx context.Context, mg *v1alpha2.Cluster) (string, error) {
	if mg.Spec.ForProvider.InstanceID != "" {
		return mg.Spec.ForProvider.InstanceID, nil
	}
	if mg.Spec.ForProvider.InstanceRef == nil || mg.Spec.ForProvider.InstanceRef.Name == "" {
		return "", fmt.Errorf("one of spec.forProvider.instanceId or spec.forProvider.instanceRef must be set")
	}

	inst := &v1alpha2.Instance{}
	key := k8stypes.NamespacedName{Name: mg.Spec.ForProvider.InstanceRef.Name, Namespace: mg.GetNamespace()}
	if err := e.Kube.Get(ctx, key, inst); err != nil {
		return "", fmt.Errorf("could not resolve InstanceRef %s/%s: %w", key.Namespace, key.Name, err)
	}
	if inst.Status.AtProvider.ID != "" {
		return inst.Status.AtProvider.ID, nil
	}

	// The referenced Instance has not yet reported its ID — fall back
	// to looking it up by name on the Akuity API. This covers the
	// bootstrapping window between Instance.Create and its first
	// Observe.
	ai, err := e.Client.GetInstance(ctx, inst.Spec.ForProvider.Name)
	if err != nil {
		return "", fmt.Errorf("could not resolve Instance %q on Akuity API: %w", inst.Spec.ForProvider.Name, err)
	}
	return ai.GetId(), nil
}
