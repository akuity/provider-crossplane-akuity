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

// Package kargoagent is the v1alpha2 KargoAgent controller. It drives
// the Akuity Kargo-plane agent endpoints (Create/Update/Get/Delete)
// and publishes the Akuity-generated install manifests as a
// connection-secret payload for downstream Compositions to consume.
package kargoagent

import (
	"context"
	"fmt"
	"strings"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
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
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// ConnectionKeyManifests is the connection-secret key holding the
// Akuity-generated agent install manifests.
const ConnectionKeyManifests = "manifests"

// kargoAgentDriftOpts filters fields that are write-only on the
// current api-client-go proto. Empty post the v0.29.1 bump which added
// proto read support for PodInheritMetadata and AutoscalerConfig;
// retained as an extension point.
var kargoAgentDriftOpts = []cmp.Option{}

// Setup registers the controller with the manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha2.KargoAgentGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

	conn := &base.Connector[*v1alpha2.KargoAgent]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha2.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha2.KargoAgent] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.KargoAgentGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha2.KargoAgent](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.KargoAgent{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha2.KargoAgent) (managed.ExternalObservation, error) {
	instanceID, err := e.resolveKargoInstanceID(ctx, mg)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	mg.Spec.ForProvider.KargoInstanceID = instanceID

	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	agent, obs, done, err := e.fetchAgent(ctx, mg, instanceID)
	if done {
		return obs, err
	}

	actual := apiToSpec(mg.Spec.ForProvider, agent)
	mg.Status.AtProvider = apiToObservation(agent)
	if mg.Status.AtProvider.HealthStatus.Code != 1 {
		mg.SetConditions(xpv1.Unavailable())
	} else {
		mg.SetConditions(xpv1.Available())
	}

	// Always compare spec so a matching desired/observed pair does not
	// trigger Update() on every poll while reconciliation is still
	// pending. Returning ResourceUpToDate=false during provisioning
	// caused a hot-loop of UpdateKargoInstanceAgent calls on the Akuity
	// API.
	upToDate := cmp.Equal(mg.Spec.ForProvider, actual, kargoAgentDriftOpts...)
	if !upToDate {
		e.Logger.Debug("KargoAgent drift detected", "diff", cmp.Diff(mg.Spec.ForProvider, actual, kargoAgentDriftOpts...))
	}

	observation := managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}

	// Manifests are only fetched once reconciliation is terminal.
	reconCode := agent.GetReconciliationStatus().GetCode()
	if reconCode == reconv1.StatusCode_STATUS_CODE_SUCCESSFUL || reconCode == reconv1.StatusCode_STATUS_CODE_FAILED {
		manifests, err := e.Client.GetKargoInstanceAgentManifestsOnce(ctx, instanceID, agent.GetId())
		if err != nil {
			mg.SetConditions(xpv1.ReconcileError(err))
			return managed.ExternalObservation{}, err
		}
		observation.ConnectionDetails = managed.ConnectionDetails{ConnectionKeyManifests: []byte(manifests)}
	}

	return observation, nil
}

// fetchAgent wraps e.Client.GetKargoInstanceAgent and folds the
// transient/error classifications into an ExternalObservation ready
// to return. See (cluster).fetchCluster for the shape rationale.
func (e *external) fetchAgent(ctx context.Context, mg *v1alpha2.KargoAgent, instanceID string) (*kargov1.KargoAgent, managed.ExternalObservation, bool, error) {
	agent, err := e.Client.GetKargoInstanceAgent(ctx, instanceID, meta.GetExternalName(mg))
	if err == nil {
		return agent, managed.ExternalObservation{}, false, nil
	}
	if reason.IsNotFound(err) {
		return nil, managed.ExternalObservation{ResourceExists: false}, true, nil
	}
	if reason.IsProvisioningWait(err) {
		mg.SetConditions(xpv1.Unavailable())
		return nil, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, true, nil
	}
	mg.SetConditions(xpv1.ReconcileError(err))
	return nil, managed.ExternalObservation{}, true, err
}

func (e *external) Create(ctx context.Context, mg *v1alpha2.KargoAgent) (managed.ExternalCreation, error) {
	instanceID, err := e.resolveKargoInstanceID(ctx, mg)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	mg.Spec.ForProvider.KargoInstanceID = instanceID

	data, err := agentDataPB(mg.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	req := &kargov1.CreateKargoInstanceAgentRequest{
		InstanceId:  instanceID,
		Name:        mg.Spec.ForProvider.Name,
		Description: agentDescription(mg.Spec.ForProvider),
		Data:        data,
		WorkspaceId: mg.Spec.ForProvider.Workspace,
	}
	agent, err := e.Client.CreateKargoInstanceAgent(ctx, req)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, agent.GetName())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha2.KargoAgent) (managed.ExternalUpdate, error) {
	instanceID, err := e.resolveKargoInstanceID(ctx, mg)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	// UpdateKargoInstanceAgent keys by agent ID — look it up first.
	existing, err := e.Client.GetKargoInstanceAgent(ctx, instanceID, meta.GetExternalName(mg))
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	data, err := agentDataPB(mg.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	req := &kargov1.UpdateKargoInstanceAgentRequest{
		InstanceId:  instanceID,
		Id:          existing.GetId(),
		Description: agentDescription(mg.Spec.ForProvider),
		Data:        data,
		WorkspaceId: mg.Spec.ForProvider.Workspace,
	}
	_, err = e.Client.UpdateKargoInstanceAgent(ctx, req)
	return managed.ExternalUpdate{}, err
}

func (e *external) Delete(ctx context.Context, mg *v1alpha2.KargoAgent) (managed.ExternalDelete, error) {
	name := meta.GetExternalName(mg)
	if name == "" {
		return managed.ExternalDelete{}, nil
	}
	instanceID, err := e.resolveKargoInstanceID(ctx, mg)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	if err := e.Client.DeleteKargoInstanceAgent(ctx, instanceID, name); err != nil {
		// The Akuity API rejects agent deletion while the agent is
		// still connected to a managed cluster; surface the error as
		// reason.Retryable so controller-runtime requeues with
		// backoff instead of failing the reconcile terminally.
		if isConnectedAgentDeleteError(err) {
			return managed.ExternalDelete{}, reason.AsRetryable(err)
		}
		return managed.ExternalDelete{}, err
	}
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(ctx context.Context) error { return nil }

// resolveKargoInstanceID returns the Akuity ID of the owning Kargo
// instance (either the explicit ID or via same-namespace KargoInstanceRef).
func (e *external) resolveKargoInstanceID(ctx context.Context, mg *v1alpha2.KargoAgent) (string, error) {
	if mg.Spec.ForProvider.KargoInstanceID != "" {
		return mg.Spec.ForProvider.KargoInstanceID, nil
	}
	if mg.Spec.ForProvider.KargoInstanceRef == nil || mg.Spec.ForProvider.KargoInstanceRef.Name == "" {
		return "", fmt.Errorf("one of spec.forProvider.kargoInstanceId or spec.forProvider.kargoInstanceRef must be set")
	}

	ki := &v1alpha2.KargoInstance{}
	key := k8stypes.NamespacedName{Name: mg.Spec.ForProvider.KargoInstanceRef.Name, Namespace: mg.GetNamespace()}
	if err := e.Kube.Get(ctx, key, ki); err != nil {
		return "", fmt.Errorf("could not resolve KargoInstanceRef %s/%s: %w", key.Namespace, key.Name, err)
	}
	if ki.Status.AtProvider.ID != "" {
		return ki.Status.AtProvider.ID, nil
	}
	// Bootstrapping fallback: resolve by name via the Akuity API.
	remote, err := e.Client.GetKargoInstance(ctx, ki.Spec.ForProvider.Name)
	if err != nil {
		return "", fmt.Errorf("could not resolve KargoInstance %q on Akuity API: %w", ki.Spec.ForProvider.Name, err)
	}
	return remote.GetId(), nil
}

func agentDescription(p v1alpha2.KargoAgentParameters) string {
	return p.Description
}

// isConnectedAgentDeleteError recognises the Akuity API error surface
// that indicates a KargoAgent cannot be deleted because a managed
// cluster is still connected to it. The exact message is API-driven
// and subject to wording drift — match loosely on the stable
// substring.
func isConnectedAgentDeleteError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connected") && strings.Contains(msg, "agent")
}
