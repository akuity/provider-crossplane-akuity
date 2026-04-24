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

// Package kargoagent is the KargoAgent controller. It drives
// the Akuity Kargo-plane agent endpoints (Create/Update/Get/Delete)
// and publishes the Akuity-generated install manifests as a
// connection-secret payload for downstream Compositions to consume.
package kargoagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	reconv1 "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/event"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
)

// ConnectionKeyManifests is the connection-secret key holding the
// Akuity-generated agent install manifests.
const ConnectionKeyManifests = "manifests"

// driftSpec is the KargoAgent drift-detection recipe. The Ignore slot
// is an extension point for write-only proto fields; it is empty post
// the v0.29.1 api-client-go bump which added proto read support for
// PodInheritMetadata and AutoscalerConfig. The shared DriftSpec
// contributes utilcmp.EquateEmpty() so nil-vs-empty collections
// resolve equal — an earlier bare cmp.Equal call flagged these as
// perpetual drift.
//
// Normalize absorbs server-defaults on spec-optional fields
// (§6 #11 audit): Namespace, Size, and Data.ArgocdNamespace all come
// back populated from the Akuity gateway (Namespace inherits
// KargoInstance.Name, Size defaults to KARGO_AGENT_SIZE_SMALL,
// ArgocdNamespace defaults to "argocd") even when the CR omits them.
// Terraform's resource_akp_kargoagent.go:377-381 applies the same
// inherit-when-empty pattern to ArgocdNamespace and MaintenanceModeExpiry.
// Without this normalize every poll sees desired="" vs observed=<default>
// and fires ApplyKargoInstance wastefully; the server Equals() short-
// circuits the DB-gen bump but the local counter still advances 1/poll.
func driftSpec() base.DriftSpec[v1alpha1.KargoAgentParameters] {
	return base.DriftSpec[v1alpha1.KargoAgentParameters]{
		Normalize: func(desired, observed *v1alpha1.KargoAgentParameters) {
			if desired == nil || observed == nil {
				return
			}
			if desired.Namespace == "" {
				desired.Namespace = observed.Namespace
			}
			if desired.KargoAgentSpec.Data.Size == "" {
				desired.KargoAgentSpec.Data.Size = observed.KargoAgentSpec.Data.Size
			}
			if desired.KargoAgentSpec.Data.ArgocdNamespace == "" {
				desired.KargoAgentSpec.Data.ArgocdNamespace =
					observed.KargoAgentSpec.Data.ArgocdNamespace
			}
		},
	}
}

// Setup registers the controller with the manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.KargoAgentGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

	conn := &base.Connector[*v1alpha1.KargoAgent]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewLegacyProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha1.KargoAgent] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.KargoAgentGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha1.KargoAgent](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.KargoAgent{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha1.KargoAgent) (managed.ExternalObservation, error) { //nolint:gocyclo
	defer base.PropagateObservedGeneration(mg)
	instanceID, err := e.resolveKargoInstanceID(ctx, mg)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	mg.Spec.ForProvider.KargoInstanceID = instanceID

	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	agent, err := e.Client.GetKargoInstanceAgent(ctx, instanceID, meta.GetExternalName(mg))
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

	actual := apiToSpec(mg.Spec.ForProvider, agent)
	mg.Status.AtProvider = observation.KargoAgent(agent)
	base.SetHealthCondition(mg, mg.Status.AtProvider.HealthStatus.Code == 1)

	// Drift compares against the ExportKargoInstance round-trippable
	// spec (the same wire shape that ApplyKargoInstance encodes),
	// matching the pattern Instance/Cluster/KargoInstance use:
	// read-via-Export, write-via-Apply. The GetKargoInstanceAgent call
	// above stays load-bearing for observation (health / reconciliation
	// / ID) — Export returns only spec. If Export fails or the agent is
	// missing from its Agents list we fall back to the Get-derived spec
	// so the reconcile still completes; the next poll re-attempts Export.
	//
	// Always compare spec so a matching desired/observed pair does not
	// trigger Update() on every poll while reconciliation is still
	// pending. Returning ResourceUpToDate=false during provisioning
	// caused a hot-loop of ApplyKargoInstance calls on the Akuity API.
	driftTarget := actual
	if exportAgent, found, xerr := e.exportedAgentSpec(ctx, instanceID, mg.Spec.ForProvider.Name, mg.Spec.ForProvider); xerr != nil {
		e.Logger.Debug("ExportKargoInstance failed; falling back to GetKargoInstanceAgent for drift", "err", xerr)
	} else if found {
		driftTarget = exportAgent
	}
	spec := driftSpec()
	desired := mg.Spec.ForProvider
	upToDate, err := base.EvaluateDrift(ctx, spec, &desired, &driftTarget, e.Logger, "KargoAgent")
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	obs := managed.ExternalObservation{
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
		obs.ConnectionDetails = managed.ConnectionDetails{ConnectionKeyManifests: []byte(manifests)}
	}

	return obs, nil
}

// apply routes both Create and Update through ApplyKargoInstance with
// only the Agents slice populated. The backend narrow-merges into the
// parent KargoInstance: sibling fields (Kargo envelope, Projects,
// Warehouses, Stages, RepoCredentials, …) are untouched so this MR
// coexists with a KargoInstance MR owning those. Same pattern Cluster
// uses against ApplyInstance (cluster/convert.go BuildApplyInstanceRequest).
func (e *external) apply(ctx context.Context, mg *v1alpha1.KargoAgent) error {
	instanceID, err := e.resolveKargoInstanceID(ctx, mg)
	if err != nil {
		return err
	}
	mg.Spec.ForProvider.KargoInstanceID = instanceID
	req, err := BuildApplyKargoInstanceRequest(instanceID, mg.Spec.ForProvider)
	if err != nil {
		return err
	}
	return e.Client.ApplyKargoInstance(ctx, req)
}

func (e *external) Create(ctx context.Context, mg *v1alpha1.KargoAgent) (managed.ExternalCreation, error) {
	defer base.PropagateObservedGeneration(mg)
	if err := e.apply(ctx, mg); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.Spec.ForProvider.Name)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha1.KargoAgent) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)
	return managed.ExternalUpdate{}, e.apply(ctx, mg)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha1.KargoAgent) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)
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

func (e *external) Disconnect(_ context.Context) error { return nil }

// exportedAgentSpec returns the canonical KargoAgentParameters for
// agentName built from ExportKargoInstance's response. `found` is
// false when Export succeeds but the agent is absent from the Agents
// slice (e.g. server lag mid-provisioning); caller falls back to
// Get-based drift in that case. Errors are transient Export failures.
func (e *external) exportedAgentSpec(ctx context.Context, instanceID, agentName string, desired v1alpha1.KargoAgentParameters) (v1alpha1.KargoAgentParameters, bool, error) {
	// ExportKargoInstance is workspace-scoped at the HTTP gateway; pass
	// empty workspace here because the KargoAgent spec doesn't carry
	// one. If Export 404s on a multi-workspace org the caller falls
	// back to GetKargoInstanceAgent-based drift, which has a
	// workspace-less path.
	exp, err := e.Client.ExportKargoInstance(ctx, instanceID, "")
	if err != nil {
		return v1alpha1.KargoAgentParameters{}, false, err
	}
	for _, entry := range exp.GetAgents() {
		if entry == nil {
			continue
		}
		wire := &akuitytypes.KargoAgent{}
		raw, merr := entry.MarshalJSON()
		if merr != nil {
			return v1alpha1.KargoAgentParameters{}, false, fmt.Errorf("encode exported agent: %w", merr)
		}
		if uerr := json.Unmarshal(raw, wire); uerr != nil {
			return v1alpha1.KargoAgentParameters{}, false, fmt.Errorf("decode exported agent: %w", uerr)
		}
		if wire.GetName() != agentName {
			continue
		}
		return wireToSpec(desired, wire), true, nil
	}
	return v1alpha1.KargoAgentParameters{}, false, nil
}

// resolveKargoInstanceID returns the Akuity ID of the owning Kargo
// instance (either the explicit ID or via same-namespace KargoInstanceRef).
func (e *external) resolveKargoInstanceID(ctx context.Context, mg *v1alpha1.KargoAgent) (string, error) {
	if mg.Spec.ForProvider.KargoInstanceID != "" {
		return mg.Spec.ForProvider.KargoInstanceID, nil
	}
	if mg.Spec.ForProvider.KargoInstanceRef == nil || mg.Spec.ForProvider.KargoInstanceRef.Name == "" {
		return "", fmt.Errorf("one of spec.forProvider.kargoInstanceId or spec.forProvider.kargoInstanceRef must be set")
	}

	ki := &v1alpha1.KargoInstance{}
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
