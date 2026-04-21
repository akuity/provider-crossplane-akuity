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

// Package cluster is the v1alpha2 Cluster controller. It replaces the
// legacy agent-install side-effect (direct kubeconfig apply) with a
// connection-details-first design: the Akuity-generated install
// manifests are published under the managed resource's connection
// secret. A downstream provider-kubernetes Composition resource
// consumes them and applies them to the target cluster.
package cluster

import (
	"context"
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
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/event"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	apisv1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert/glue"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// ConnectionKeyManifests is the connection-secret key under which the
// Akuity-generated install manifests are published. Downstream
// Compositions consume the value as YAML.
const ConnectionKeyManifests = "manifests"

// clusterDriftOpts filters fields that are write-only on the current
// api-client-go proto — i.e. the controller can send them on apply but
// cannot read them back, so including them in cmp.Equal would drift
// flap forever. Empty post the v0.29.1 bump which added proto read
// support for PodInheritMetadata; retained as an extension point.
var clusterDriftOpts = []cmp.Option{}

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
	actual := apiToSpec(instanceID, mg.Spec.ForProvider, ac)
	mg.Status.AtProvider = apiToObservation(ac)

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

	// Manifests are only fetched once reconciliation is terminal; a
	// fetch before that returns partial content.
	reconCode := ac.GetReconciliationStatus().GetCode()
	if reconCode == reconv1.StatusCode_STATUS_CODE_SUCCESSFUL || reconCode == reconv1.StatusCode_STATUS_CODE_FAILED {
		manifests, err := e.Client.GetClusterManifestsOnce(ctx, instanceID, ac.GetId())
		if err != nil {
			mg.SetConditions(xpv1.ReconcileError(err))
			return managed.ExternalObservation{}, err
		}
		observation.ConnectionDetails = managed.ConnectionDetails{
			ConnectionKeyManifests: []byte(manifests),
		}
	}

	return observation, nil
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
	if err := e.apply(ctx, mg); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.Spec.ForProvider.Name)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha2.Cluster) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, e.apply(ctx, mg)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha2.Cluster) (managed.ExternalDelete, error) {
	name := meta.GetExternalName(mg)
	if name == "" {
		return managed.ExternalDelete{}, nil
	}
	if err := e.Client.DeleteCluster(ctx, mg.Spec.ForProvider.InstanceID, name); err != nil {
		return managed.ExternalDelete{}, fmt.Errorf("could not delete cluster: %w", err)
	}
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error { return nil }

// apply is shared by Create and Update — the Akuity ApplyCluster call
// is idempotent.
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
	return nil
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
