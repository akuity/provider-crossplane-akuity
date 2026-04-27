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

// Package kargodefaultshardagent is the KargoDefaultShardAgent
// controller. It owns the single kargoInstanceSpec.defaultShardAgent
// field of a Kargo instance via the PatchKargoInstance endpoint,
// which keys by opaque Akuity ID and server-side-merges into only
// the provided sub-tree.
package kargodefaultshardagent

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
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

// Setup registers the controller.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.KargoDefaultShardAgentGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

	conn := &base.Connector[*v1alpha1.KargoDefaultShardAgent]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewLegacyProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha1.KargoDefaultShardAgent] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.KargoDefaultShardAgentGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha1.KargoDefaultShardAgent](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
		managed.WithManagementPolicies(),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.KargoDefaultShardAgent{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha1.KargoDefaultShardAgent) (managed.ExternalObservation, error) { //nolint:gocyclo // observe branches are independent and flat; splitting hurts readability
	defer base.PropagateObservedGeneration(mg)
	kargoID, err := e.resolveKargoID(ctx, mg)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	// Short-circuit on a cached terminal write before any gateway round-
	// trip. With NameAsExternalName the external-name is stamped before
	// Create runs, so a Patch that fails terminally on bad input would
	// otherwise loop GetKargoInstanceByID->Patch reject at controller-
	// runtime backoff (~2s). Resolving the desired agent ID up front
	// keys the guard on the same payload Create/Update use; an unresolved
	// agent is left to fall through so the existing missing-agent retry
	// path stays intact.
	if e.HasTerminalWriteResource(mg, v1alpha1.KargoDefaultShardAgentGroupVersionKind) {
		desiredID, derr := e.resolveDesiredAgentID(ctx, kargoID, mg.Spec.ForProvider.AgentName)
		if derr == nil {
			if obs, err, ok := e.suppressTerminalWrite(mg, kargoID, desiredID); ok {
				return obs, err
			}
		}
	}
	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ki, err := e.Client.GetKargoInstanceByID(ctx, kargoID)
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

	// Server stores the agent's opaque ID on spec.defaultShardAgent;
	// resolve the spec.forProvider.AgentName to the same ID space so
	// the compare is apples-to-apples. Skipping the resolve on an
	// empty desired (clear-pin path) avoids a pointless List.
	observedID := ki.GetSpec().GetDefaultShardAgent()
	var desiredID string
	if mg.Spec.ForProvider.AgentName != "" {
		resolved, rerr := e.resolveDesiredAgentID(ctx, kargoID, mg.Spec.ForProvider.AgentName)
		if rerr != nil {
			// Missing agent means the pin cannot exist yet. Let Create
			// retry once the agent appears; surface other errors.
			if reason.IsNotFound(rerr) {
				return managed.ExternalObservation{ResourceExists: false}, nil
			}
			return managed.ExternalObservation{}, rerr
		}
		desiredID = resolved
	}

	mg.Status.AtProvider = v1alpha1.KargoDefaultShardAgentObservation{
		AgentName:       mg.Spec.ForProvider.AgentName,
		KargoInstanceID: kargoID,
	}
	base.SetHealthCondition(mg, true)

	// Deletion fast-path: once the MR has a deletionTimestamp, Delete()
	// clears defaultShardAgent on the server (empty string). The next
	// Observe sees observedID="" but desiredID is still the agent's
	// opaque ID, so the compare below flips ResourceUpToDate=false and
	// runtime dispatches Delete every poll. Report ResourceExists=false
	// once the server-side pin is already cleared so the managed
	// reconciler drops the finalizer.
	if meta.WasDeleted(mg) && observedID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	upToDate := observedID == desiredID
	if !upToDate {
		e.Logger.Debug("KargoDefaultShardAgent drift detected",
			"observedID", observedID, "desiredID", desiredID, "agentName", mg.Spec.ForProvider.AgentName)
		if obs, err, ok := e.suppressTerminalWrite(mg, kargoID, desiredID); ok {
			return obs, err
		}
	} else {
		e.clearTerminalWrite(mg, kargoID, desiredID)
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mg *v1alpha1.KargoDefaultShardAgent) (managed.ExternalCreation, error) {
	defer base.PropagateObservedGeneration(mg)
	if err := e.patch(ctx, mg, mg.Spec.ForProvider.AgentName); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.GetName())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha1.KargoDefaultShardAgent) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)
	return managed.ExternalUpdate{}, e.patch(ctx, mg, mg.Spec.ForProvider.AgentName)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha1.KargoDefaultShardAgent) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)
	e.ClearTerminalWriteResource(mg, v1alpha1.KargoDefaultShardAgentGroupVersionKind)

	// Clearing the default is the API-neutral "no pinning" signal; we
	// intentionally do not delete the Kargo instance itself.
	return managed.ExternalDelete{}, e.patch(ctx, mg, "")
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// patch writes the desired defaultShardAgent via the narrow
// PatchKargoInstance endpoint. The server merges into only the
// provided sub-tree and stores the agent's opaque ID (not its name)
// as the defaultShardAgent value.
// Empty string clears the pin.
//
// Envelope is `{"spec": {"defaultShardAgent": "<agent-id>"}}`, not
// `{"spec": {"kargoInstanceSpec": {"defaultShardAgent": ...}}}`,
// which is what an earlier iteration sent; the server's patch
// unmarshaler rejected the extra "kargoInstanceSpec" envelope with
// `unknown field "kargoInstanceSpec"`.
func (e *external) patch(ctx context.Context, mg *v1alpha1.KargoDefaultShardAgent, desiredAgentName string) error {
	kargoID, err := e.resolveKargoID(ctx, mg)
	if err != nil {
		return err
	}

	// desiredAgentName == "" is the "clear pin" path (Delete); skip
	// name-to-ID resolution.
	value := ""
	if desiredAgentName != "" {
		resolved, rerr := e.resolveDesiredAgentID(ctx, kargoID, desiredAgentName)
		if rerr != nil {
			return fmt.Errorf("resolve default-shard agent %q: %w", desiredAgentName, rerr)
		}
		value = resolved
	}
	key, err := kargoDefaultShardAgentTerminalWriteKey(mg, kargoID, value)
	if err != nil {
		return err
	}

	patch, err := structpb.NewStruct(map[string]any{
		"spec": map[string]any{
			"defaultShardAgent": value,
		},
	})
	if err != nil {
		return fmt.Errorf("build defaultShardAgent patch: %w", err)
	}

	if err := e.Client.PatchKargoInstance(ctx, kargoID, patch); err != nil {
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

func (e *external) suppressTerminalWrite(mg *v1alpha1.KargoDefaultShardAgent, kargoID, desiredID string) (managed.ExternalObservation, error, bool) {
	if e.TerminalWrites == nil {
		return managed.ExternalObservation{}, nil, false
	}
	key, err := kargoDefaultShardAgentTerminalWriteKey(mg, kargoID, desiredID)
	if err != nil {
		return e.SkipTerminalWriteGuard(err)
	}
	return e.SuppressTerminalWrite(mg, key)
}

func kargoDefaultShardAgentTerminalWriteKey(mg *v1alpha1.KargoDefaultShardAgent, kargoID, desiredID string) (base.TerminalWriteKey, error) {
	return base.NewTerminalWriteKey(mg, v1alpha1.KargoDefaultShardAgentGroupVersionKind, kargoID, mg.Spec.ForProvider.AgentName, desiredID)
}

func (e *external) clearTerminalWrite(mg *v1alpha1.KargoDefaultShardAgent, kargoID, desiredID string) {
	if !e.HasTerminalWriteResource(mg, v1alpha1.KargoDefaultShardAgentGroupVersionKind) {
		return
	}
	key, err := kargoDefaultShardAgentTerminalWriteKey(mg, kargoID, desiredID)
	if err != nil {
		e.LogTerminalWriteGuardSkipped(err)
		return
	}
	e.ClearTerminalWrite(key)
}

func (e *external) resolveDesiredAgentID(ctx context.Context, kargoID, name string) (string, error) {
	if name == "" {
		return "", nil
	}
	agent, err := e.Client.GetKargoInstanceAgent(ctx, kargoID, name)
	if err != nil {
		return "", err
	}
	return agent.GetId(), nil
}

// resolveKargoID returns the opaque Akuity ID of the target Kargo
// instance. ForProvider.KargoInstanceID takes precedence; if absent,
// KargoInstanceRef is resolved against a KargoInstance MR in the same
// namespace and its Status.AtProvider.ID is used.
//
// The cached Status.AtProvider.KargoInstanceID is consulted only as a
// last-resort during deletion when the referenced KargoInstance MR has
// itself been removed (typical composition teardown). It is NOT used
// to paper over transient kube-apiserver errors or a pending new
// parent, since that would let a live ref retarget silently keep
// patching the previous Kargo instance.
func (e *external) resolveKargoID(ctx context.Context, mg *v1alpha1.KargoDefaultShardAgent) (string, error) {
	if id := mg.Spec.ForProvider.KargoInstanceID; id != "" {
		return id, nil
	}
	if mg.Spec.ForProvider.KargoInstanceRef == nil || mg.Spec.ForProvider.KargoInstanceRef.Name == "" {
		return "", fmt.Errorf("one of spec.forProvider.kargoInstanceId or spec.forProvider.kargoInstanceRef must be set")
	}
	ki := &v1alpha1.KargoInstance{}
	key := k8stypes.NamespacedName{Name: mg.Spec.ForProvider.KargoInstanceRef.Name, Namespace: mg.GetNamespace()}
	if err := e.Kube.Get(ctx, key, ki); err != nil {
		if apierrors.IsNotFound(err) && meta.WasDeleted(mg) {
			if cached := mg.Status.AtProvider.KargoInstanceID; cached != "" {
				return cached, nil
			}
		}
		return "", fmt.Errorf("could not resolve KargoInstanceRef %s/%s: %w", key.Namespace, key.Name, err)
	}
	if ki.Status.AtProvider.ID == "" {
		return "", fmt.Errorf("referenced KargoInstance %s/%s has not yet reported an ID; waiting for its controller to observe", key.Namespace, key.Name)
	}
	return ki.Status.AtProvider.ID, nil
}
