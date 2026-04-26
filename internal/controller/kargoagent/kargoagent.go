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

// Package kargoagent reconciles KargoAgent managed resources. It drives
// the Akuity Kargo-plane agent endpoints and installs Akuity-generated
// agent manifests onto managed clusters.
package kargoagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/kube"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
)

// driftSpec is the KargoAgent drift-detection recipe. The shared
// DriftSpec contributes utilcmp.EquateEmpty() so nil-vs-empty
// collections resolve equal.
//
// Normalize absorbs server defaults on optional spec fields. Namespace,
// Size, ArgocdNamespace, and TargetVersion all come back populated from
// the Akuity gateway even when the CR omits them. Without this normalize,
// every poll sees desired="" vs observed=<default> and fires
// ApplyKargoInstance wastefully; the server Equals() short-circuits the
// DB generation bump, but the local counter still advances 1/poll.
//
// AkuityManaged is server-authority after create. The create path
// persists whatever the client sends, but update copies the existing DB
// value and ignores incoming changes. Without normalization, a
// post-create flip to true on a row created with false produces
// desired=&true vs observed=nil (proto3 bool, omitempty wire, nil
// *bool), drift fires every poll, ApplyKargoInstance hits 1/poll, and
// the guarded SQL trigger keeps DB gen frozen. Copy observed into
// desired so the pair matches the server-retained value; Normalize only
// runs on Observe, so the user's create-time AkuityManaged still writes
// through unchanged.
//
// TargetVersion is server-defaulted to the latest agent version when
// the CR leaves it empty. Same shape as ArgocdNamespace: inherit
// observed when desired is empty so unset does not round-trip as
// perpetual drift.
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
			if desired.KargoAgentSpec.Data.TargetVersion == "" {
				desired.KargoAgentSpec.Data.TargetVersion =
					observed.KargoAgentSpec.Data.TargetVersion
			}
			// Kustomization round-trips as a verbatim string; the
			// platform appends a trailing "\n" to user values that
			// don't already have one. Same wasteful-Apply pattern as
			// the other normalize entries: server Equals() short-
			// circuits the DB-gen bump but the local counter still
			// advances 1/poll. Flatten newline-only differences so
			// presence-mode-style equality holds for byte-identical-
			// modulo-newline content.
			if strings.TrimRight(desired.KargoAgentSpec.Data.Kustomization, "\n") ==
				strings.TrimRight(observed.KargoAgentSpec.Data.Kustomization, "\n") &&
				desired.KargoAgentSpec.Data.Kustomization !=
					observed.KargoAgentSpec.Data.Kustomization {
				desired.KargoAgentSpec.Data.Kustomization =
					observed.KargoAgentSpec.Data.Kustomization
			}
			desired.KargoAgentSpec.Data.AkuityManaged =
				observed.KargoAgentSpec.Data.AkuityManaged
			// The gateway currently drops PodInheritMetadata and
			// AutoscalerConfig on Apply: the platform builds the agent
			// data without copying these two fields from the Apply
			// request, so CreateKargoInstanceAgent and
			// UpdateKargoInstanceAgent never see the user's value. Without
			// normalization, the CR stays desired=<value> while observed
			// stays nil/zero, firing ApplyKargoInstance every poll. Copy
			// observed into desired so drift detection does not chase
			// fields the platform ignores; a platform fix can remove this
			// without CR churn because observed will start reflecting the
			// user's value and equality will hold.
			desired.KargoAgentSpec.Data.PodInheritMetadata =
				observed.KargoAgentSpec.Data.PodInheritMetadata
			desired.KargoAgentSpec.Data.AutoscalerConfig =
				observed.KargoAgentSpec.Data.AutoscalerConfig
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
	// Mirror the observed agent sub-tree onto AtProvider so
	// compositions and dashboards can read the effective server-side
	// shape without a separate Export call. Parity with
	// KargoInstance.AtProvider.Kargo.
	mg.Status.AtProvider.KargoAgentSpec = actual.KargoAgentSpec
	base.SetHealthCondition(mg, mg.Status.AtProvider.HealthStatus.Code == 1)

	// Drift compares against the ExportKargoInstance round-trippable
	// spec, the same wire shape ApplyKargoInstance encodes. Get remains
	// the source of observation fields such as health, reconciliation,
	// and ID. If Export fails or omits the agent, fall back to the
	// Get-derived spec so the reconcile still completes.
	//
	// Always compare spec so a matching desired/observed pair does not
	// trigger Update() on every poll while reconciliation is still
	// pending. Returning ResourceUpToDate=false during provisioning
	// caused a hot-loop of ApplyKargoInstance calls on the Akuity API.
	driftTarget := actual
	if exportAgent, found, xerr := e.exportedAgentSpec(ctx, mg, instanceID); xerr != nil {
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

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

// apply routes both Create and Update through ApplyKargoInstance with
// only the Agents slice populated. The backend narrow-merges into the
// parent KargoInstance: sibling fields (Kargo envelope, Projects,
// Warehouses, Stages, RepoCredentials, etc.) are untouched so this MR
// coexists with a KargoInstance MR owning those. Same pattern Cluster
// uses against ApplyInstance (cluster/convert.go BuildApplyInstanceRequest).
func (e *external) apply(ctx context.Context, mg *v1alpha1.KargoAgent) error {
	instanceID, err := e.resolveKargoInstanceID(ctx, mg)
	if err != nil {
		return err
	}
	mg.Spec.ForProvider.KargoInstanceID = instanceID
	// ApplyKargoInstance is workspace-scoped at the HTTP gateway
	// (`/orgs/{org}/workspaces/{workspace_id}/kargo/instances/{id}/apply`);
	// an unset WorkspaceId on the request substitutes empty into the
	// URL template and portal-server 404s. When the user didn't pin a
	// workspace on the KargoAgent CR, inherit it from the parent
	// KargoInstance's spec, the same MR reference the controller
	// already used to resolve the instance ID.
	if mg.Spec.ForProvider.Workspace == "" {
		mg.Spec.ForProvider.Workspace = e.resolveWorkspaceFromParent(ctx, mg)
	}
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

	// One-time apply: when a kubeconfig source is configured, install
	// the agent manifests on the managed cluster before setting the
	// external-name. Mirrors Cluster.Create: a failure here leaves the
	// external-name unset so the next reconcile retries via Create
	// rather than observing a "ready" MR with a stale target cluster.
	// Server-pushed agent upgrades require a spec change or recreate to
	// re-land on the managed cluster.
	if target := targetKubeConfig(mg.Spec.ForProvider); target.HasKubeConfig() {
		if err := e.installAgentManifests(ctx, mg, target, false); err != nil {
			return managed.ExternalCreation{}, err
		}
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

	// Remove agent resources from the managed cluster before issuing
	// the platform-side delete, so the managed cluster's kube-apiserver
	// no longer has agent workloads pointing at a soon-to-be-gone
	// platform row. Mirrors Cluster.Delete with
	// removeAgentResourcesOnDestroy=true.
	if mg.Spec.ForProvider.RemoveAgentResourcesOnDestroy {
		if target := targetKubeConfig(mg.Spec.ForProvider); target.HasKubeConfig() {
			if err := e.installAgentManifests(ctx, mg, target, true); err != nil {
				return managed.ExternalDelete{}, err
			}
		}
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

// targetKubeConfig projects the forProvider kubeconfig fields into the
// shared TargetKubeConfig struct.
func targetKubeConfig(fp v1alpha1.KargoAgentParameters) kube.TargetKubeConfig {
	return kube.TargetKubeConfig{
		EnableInCluster: fp.EnableInClusterKubeConfig,
		SecretName:      fp.KubeConfigSecretRef.Name,
		SecretNamespace: fp.KubeConfigSecretRef.Namespace,
	}
}

// installAgentManifests fetches the Akuity-generated install manifests
// for mg's agent and applies (or deletes when del is true) them onto
// the cluster identified by target. Requires mg.Spec.ForProvider
// .KargoInstanceID to be resolved.
//
// Uses the wait-for-reconciliation manifest fetcher so the first Create
// right after ApplyKargoInstance doesn't race the gateway's manifest
// renderer and hit codes.Unavailable.
// A one-shot failure here would leave the platform row stamped without
// the agent installed on the managed cluster because
// Update does not retry manifest installation.
func (e *external) installAgentManifests(ctx context.Context, mg *v1alpha1.KargoAgent, target kube.TargetKubeConfig, del bool) error {
	instanceID := mg.Spec.ForProvider.KargoInstanceID
	manifests, err := e.Client.GetKargoInstanceAgentManifests(ctx, instanceID, mg.Spec.ForProvider.Name)
	if err != nil {
		return fmt.Errorf("could not get kargo agent manifests to %s: %w", applyVerb(del), err)
	}
	if err := kube.ApplyManifestsToTarget(ctx, e.Kube, e.Logger, target, manifests, del); err != nil {
		return fmt.Errorf("could not %s kargo agent manifests: %w", applyVerb(del), err)
	}
	return nil
}

func applyVerb(del bool) string {
	if del {
		return "delete"
	}
	return "apply"
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// exportedAgentSpec returns the canonical KargoAgentParameters for
// mg.Spec.ForProvider.Name built from ExportKargoInstance's response.
// `found` is false when Export succeeds but the agent is absent from
// the Agents slice (e.g. server lag mid-provisioning); caller falls
// back to Get-based drift in that case. Errors are transient Export
// failures.
func (e *external) exportedAgentSpec(ctx context.Context, mg *v1alpha1.KargoAgent, instanceID string) (v1alpha1.KargoAgentParameters, bool, error) {
	// ExportKargoInstance is workspace-scoped at the HTTP gateway; an
	// empty workspace 404s on multi-workspace orgs. Resolve the same
	// way apply() does: prefer the spec value, fall back to the parent
	// KargoInstance's workspace. Keeps Export on the happy path so the
	// GetKargoInstanceAgent fallback only triggers on real Export
	// failures rather than a deterministic 404.
	workspace := mg.Spec.ForProvider.Workspace
	if workspace == "" {
		workspace = e.resolveWorkspaceFromParent(ctx, mg)
	}
	exp, err := e.Client.ExportKargoInstance(ctx, instanceID, workspace)
	if err != nil {
		return v1alpha1.KargoAgentParameters{}, false, err
	}
	agentName := mg.Spec.ForProvider.Name
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
		return wireToSpec(mg.Spec.ForProvider, wire), true, nil
	}
	return v1alpha1.KargoAgentParameters{}, false, nil
}

// resolveWorkspaceFromParent returns the workspace of the parent
// KargoInstance pointed at by mg.Spec.ForProvider.KargoInstanceRef, or
// "" when there is no ref or the lookup fails. Used by apply() and
// exportedAgentSpec() when the user did not pin a workspace on the
// KargoAgent CR. Mirrors
// the workspace resolution used by apply().
func (e *external) resolveWorkspaceFromParent(ctx context.Context, mg *v1alpha1.KargoAgent) string {
	ref := mg.Spec.ForProvider.KargoInstanceRef
	if ref == nil || ref.Name == "" {
		return ""
	}
	parent := &v1alpha1.KargoInstance{}
	key := k8stypes.NamespacedName{Name: ref.Name, Namespace: mg.GetNamespace()}
	if err := e.Kube.Get(ctx, key, parent); err != nil {
		return ""
	}
	if parent.Spec.ForProvider.Workspace != "" {
		return parent.Spec.ForProvider.Workspace
	}
	return parent.Status.AtProvider.Workspace
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
// and subject to wording drift; match loosely on the stable
// substring.
func isConnectedAgentDeleteError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connected") && strings.Contains(msg, "agent")
}
