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

// Package kargoinstance reconciles KargoInstance managed resources
// through the Akuity Kargo service gateway.
package kargoinstance

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/event"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base/children"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	"github.com/akuityio/provider-crossplane-akuity/internal/secrets"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
)

const (
	kargoCMKey     = "kargo-cm"
	kargoSecretKey = "kargo-secret"
)

// Declarative Kargo child-resource contract. Each entry in
// spec.forProvider.resources must carry one of these
// apiVersion/kind pairs; anything else is rejected during reconcile.
const (
	kargoAPIVersion              = "kargo.akuity.io/v1alpha1"
	kargoKindProject             = "Project"
	kargoKindWarehouse           = "Warehouse"
	kargoKindStage               = "Stage"
	kargoKindAnalysisTemplate    = "AnalysisTemplate"
	kargoKindPromotionTask       = "PromotionTask"
	kargoKindClusterPromotionTsk = "ClusterPromotionTask"

	coreAPIVersion = "v1"
	coreKindSecret = "Secret"

	kargoCredTypeLabel = "kargo.akuity.io/cred-type"
	kargoCredTypeGit   = "git"
	kargoCredTypeHelm  = "helm"
	kargoCredTypeGen   = "generic"
	kargoCredTypeImage = "image"

	kargoDNS1123LabelPattern = `^[a-z0-9][a-z0-9-]*$`
)

// driftSpec is the struct-level drift recipe for KargoInstance.
// Resources, KargoConfigMap, and KargoRepoCredentialSecretRefs are
// ignored here because each needs custom drift handling: additive
// semantics, hash-based rotation, or write-only gateway behavior.
//
// Normalize absorbs server-echoed fields the user hasn't pinned so the
// first-poll delta does not flap. Workspace is spec-only, while
// Subdomain and AkuityIntelligenceExtension are server-defaulted.
// Copy spec-only values onto observed, and server defaults onto
// desired, until the user pins a value.
func driftSpec() base.DriftSpec[v1alpha1.KargoInstanceParameters] {
	return base.DriftSpec[v1alpha1.KargoInstanceParameters]{
		Ignore: []cmp.Option{
			cmpopts.IgnoreFields(v1alpha1.KargoInstanceParameters{},
				"Resources", "KargoConfigMap", "KargoRepoCredentialSecretRefs"),
			// Every []string on the KargoInstance tree is set-semantic
			// on the gateway: OidcConfig.AdditionalScopes,
			// KargoInstanceSpec.{GlobalCredentialsNs,GlobalServiceAccountNs},
			// AkuityIntelligence.{AllowedUsernames,AllowedGroups},
			// and KargoPredefinedAccountClaimValue.Values. The Export
			// response always returns them sorted alphabetically; the
			// CR round-trips user order. Compare order-insensitively so
			// harmless order changes do not trigger Apply on every poll.
			cmpopts.SortSlices(func(a, b string) bool { return a < b }),
		},
		Normalize: func(desired, observed *v1alpha1.KargoInstanceParameters) {
			if desired == nil || observed == nil {
				return
			}
			// Workspace is a spec-only field; Kargo Export doesn't echo
			// it. Copy desired onto observed so the compare is neutral.
			if observed.Workspace == "" {
				observed.Workspace = desired.Workspace
			}
			// Server sets Subdomain and AkuityIntelligenceExtension
			// defaults on every KargoInstance. Inherit them into the
			// desired side when the CR didn't pin a value.
			if desired.Kargo.Subdomain == "" {
				desired.Kargo.Subdomain = observed.Kargo.Subdomain
			}
			if desired.Kargo.KargoInstanceSpec.AkuityIntelligence == nil {
				desired.Kargo.KargoInstanceSpec.AkuityIntelligence =
					observed.Kargo.KargoInstanceSpec.AkuityIntelligence
			}
			// GcConfig is server-retained: once a value has been Applied,
			// the gateway keeps it even after the CR clears the field.
			// Without this inherit the drift compare would see
			// desired=nil vs observed=<last-applied> and hot-loop Apply
			// forever. User-set values still propagate because desired
			// non-nil short-circuits this branch.
			if desired.Kargo.KargoInstanceSpec.GcConfig == nil {
				desired.Kargo.KargoInstanceSpec.GcConfig =
					observed.Kargo.KargoInstanceSpec.GcConfig
			}
			// OidcConfig follows the same server-retained shape as
			// GcConfig: once enabled, the gateway keeps issuer/client
			// IDs, scopes, and predefined-account claims even after the
			// CR removes spec.kargo.oidcConfig. Without this inherit,
			// desired=nil vs observed={issuerUrl, clientId, ...} fires
			// drift every poll, producing roughly 350 wasted Apply writes
			// in 12 minutes in staging. DexConfigSecretRef is carried
			// forward before Normalize runs, so the ref-based path is
			// unaffected.
			if desired.Kargo.OidcConfig == nil {
				desired.Kargo.OidcConfig = observed.Kargo.OidcConfig
			}
			// DefaultShardAgent is owned by the KargoDefaultShardAgent MR,
			// not the KargoInstance MR. Treat unset desired as "leave
			// alone"; otherwise KargoInstance fights KDSA because KDSA
			// writes the agent ID server-side while KargoInstance desired
			// is empty.
			if desired.Kargo.KargoInstanceSpec.DefaultShardAgent == "" {
				desired.Kargo.KargoInstanceSpec.DefaultShardAgent =
					observed.Kargo.KargoInstanceSpec.DefaultShardAgent
			}
		},
	}
}

// errSecretInKargoResources is surfaced when a user puts a v1/Secret
// entry into spec.forProvider.resources. Repo-credential Secrets
// now flow through the typed KargoRepoCredentialSecretRefs slot so
// plaintext never lives on the MR spec.
const errSecretInKargoResources = "resources[%d]: v1/Secret entries are not accepted; use spec.forProvider.kargoRepoCredentialSecretRefs"

var (
	kargoDNS1123LabelRE = regexp.MustCompile(kargoDNS1123LabelPattern)
	kargoCredTypes      = []string{kargoCredTypeGit, kargoCredTypeHelm, kargoCredTypeGen, kargoCredTypeImage}
	kargoCredTypesMsg   = strings.Join(kargoCredTypes, ", ")
)

// Setup registers the controller with the manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.KargoInstanceGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

	conn := &base.Connector[*v1alpha1.KargoInstance]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewLegacyProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha1.KargoInstance] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.KargoInstanceGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha1.KargoInstance](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.KargoInstance{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

//nolint:gocyclo // Observe coordinates struct cmp + 2 export-based drift checks + secret hash; the linear branching is the simplest readable form.
func (e *external) Observe(ctx context.Context, mg *v1alpha1.KargoInstance) (managed.ExternalObservation, error) {
	defer base.PropagateObservedGeneration(mg)
	// Short-circuit on a cached terminal write before any gateway round-
	// trip. Crossplane's NameAsExternalName initializer stamps external-
	// name early, so a Create that fails terminally on bad input
	// (workspace not found, invalid spec field, malformed Kustomization)
	// would otherwise loop GetKargoInstance->NotFound->Create->reject at
	// controller-runtime backoff (~2s). HasTerminalWriteResource is a
	// cheap map lookup so happy-path Observes only pay the secret resolve
	// when an entry exists.
	if e.HasTerminalWriteResource(mg, v1alpha1.KargoInstanceGroupVersionKind) {
		if obs, err, ok := e.suppressTerminalWrite(ctx, mg); ok {
			return obs, err
		}
	}
	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ki, err := e.Client.GetKargoInstance(ctx, meta.GetExternalName(mg))
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
	presence := base.ForProviderPresence(ctx, e.Kube, mg, v1alpha1.KargoInstanceGroupVersionKind)

	actual, err := apiToSpec(ki)
	if err != nil {
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	// Preserve controller-authored AtProvider fields across the refresh.
	// apiToObservation rebuilds the struct from gateway data, which has
	// no SecretHash; without this carry-over every Observe would zero it,
	// masking rotation until the next Create/Update writes a new value.
	prevSecretHash := mg.Status.AtProvider.SecretHash
	prevWorkspace := mg.Status.AtProvider.Workspace
	mg.Status.AtProvider = observation.KargoInstance(ki)
	mg.Status.AtProvider.SecretHash = prevSecretHash
	// GetKargoInstance echoes the canonical workspace ID; cache it on
	// AtProvider so ExportKargoInstance, Apply, and Delete can route to
	// the right workspace-scoped HTTP path without an extra
	// ListWorkspaces round-trip. Carry the prior
	// stamped value forward when the gateway response omits it.
	if ws := ki.GetWorkspaceId(); ws != "" {
		mg.Status.AtProvider.Workspace = ws
	} else {
		mg.Status.AtProvider.Workspace = prevWorkspace
	}
	base.SetHealthCondition(mg, mg.Status.AtProvider.HealthStatus.Code == 1)

	var exp *kargov1.ExportKargoInstanceResponse
	if ki.GetId() != "" {
		// ExportKargoInstance returns the same Kargo object shape that
		// ApplyKargoInstance accepts. Use it as the primary observed
		// config source for struct drift and AtProvider.Kargo. If the
		// export endpoint is transiently unavailable, fall back to Get so
		// observation still progresses and the next poll retries Export.
		exp, err = e.Client.ExportKargoInstance(ctx, ki.GetId(), mg.Status.AtProvider.Workspace)
		if err != nil {
			e.Logger.Debug("KargoInstance export failed; falling back to GetKargoInstance for primary drift", "err", err)
		} else {
			exportActual, found, xerr := exportToSpec(ki, exp)
			if xerr != nil {
				mg.SetConditions(xpv1.ReconcileError(xerr))
				return managed.ExternalObservation{}, xerr
			}
			if found {
				actual = exportActual
			}
		}
	}

	// KargoSecretRef is spec-only (the gateway doesn't round-trip
	// it); carry it forward so the struct comparison doesn't flag it
	// as drift. The SecretHash in AtProvider catches rotation.
	//
	// KargoConfigMap participates in drift independently against the
	// ExportKargoInstance response. Do not carry desired into actual,
	// or a cleared key/out-of-band edit would be masked.
	//
	// KargoResources drift is computed independently below against
	// the ExportKargoInstance response; we exclude it from the
	// struct-level comparison via cmpopts.IgnoreFields instead of
	// carrying it over, so real drift can surface.
	actual.KargoSecretRef = mg.Spec.ForProvider.KargoSecretRef

	// DexConfigSecretRef lives nested under spec.oidcConfig and is
	// spec-only; carry it forward so the struct comparison doesn't
	// flag it as drift. Inline spec.oidcConfig.dexConfigSecret values
	// DO round-trip through ExportKargoInstance, so they stay under the
	// control of the comparator.
	if desired := mg.Spec.ForProvider.Kargo.OidcConfig; desired != nil && desired.DexConfigSecretRef != nil {
		if actual.Kargo.OidcConfig == nil {
			actual.Kargo.OidcConfig = &crossplanetypes.KargoOidcConfig{}
		}
		actual.Kargo.OidcConfig.DexConfigSecretRef = desired.DexConfigSecretRef
	}

	// Mirror the primary observed Kargo sub-tree onto AtProvider
	// (Export when available, Get fallback) so compositions and
	// dashboards can read the effective server-side spec. This stays
	// controller-side: the observation projector cannot build
	// AtProvider.Kargo without reusing apiToSpec/exportToSpec, which
	// bridges structpb, wire types, and the Crossplane spec shape.
	// Moving that transform into observation would create a circular
	// import.
	mg.Status.AtProvider.Kargo = actual.Kargo

	// Struct-level comparison of the primary spec shape. Resources,
	// KargoConfigMap, and KargoRepoCredentialSecretRefs are checked
	// separately because they have additive semantics or hash-based
	// rotation that a plain struct comparison cannot express.
	structSpec := driftSpec()
	structSpec.Presence = presence
	desired := mg.Spec.ForProvider
	upToDate, err := base.EvaluateDrift(ctx, structSpec, &desired, &actual, e.Logger, "KargoInstance")
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	// Export-based drift check runs when the user has spec-level
	// opinions on either kargoConfigMap or kargoResources. A single
	// Export serves both, minimizing gateway round-trips.
	// kargoConfigMap is key-additive: desired keys must match
	// observed, platform-added keys are ignored, and removing a key
	// from spec means "stop managing this key" rather than "clear it
	// on the platform".
	needsExport := len(mg.Spec.ForProvider.KargoConfigMap) > 0 ||
		len(mg.Spec.ForProvider.Resources) > 0
	if upToDate && needsExport {
		if exp == nil {
			// A failed Export on an otherwise-healthy instance is
			// recoverable; leaving upToDate=true lets the next poll
			// retry rather than stampede Apply on a transient error.
			e.Logger.Debug("KargoInstance export unavailable; deferring child-drift check")
		} else {
			if len(mg.Spec.ForProvider.KargoConfigMap) > 0 {
				ok, observed, cerr := kargoConfigMapUpToDate(mg.Spec.ForProvider.KargoConfigMap, exp)
				if cerr != nil {
					mg.SetConditions(xpv1.ReconcileError(cerr))
					return managed.ExternalObservation{}, cerr
				}
				if !ok {
					e.Logger.Debug("kargoConfigMap drift detected",
						"desired", mg.Spec.ForProvider.KargoConfigMap, "observed", observed)
					upToDate = false
				}
			}
			if upToDate && len(mg.Spec.ForProvider.Resources) > 0 {
				ok, report, rerr := kargoResourcesUpToDate(mg.Spec.ForProvider.Resources, exp)
				if rerr != nil {
					mg.SetConditions(xpv1.ReconcileError(rerr))
					return managed.ExternalObservation{}, rerr
				}
				if !ok {
					e.Logger.Debug("resources drift detected",
						"missing", report.Missing, "changed", report.Changed)
					upToDate = false
				}
			}
		}
	}

	if upToDate {
		sec, serr := resolveKargoSecrets(ctx, e.Kube, mg)
		if serr != nil {
			mg.SetConditions(xpv1.ReconcileError(serr))
			return managed.ExternalObservation{}, serr
		}
		if sec.Hash() != getSecretHash(mg) {
			e.Logger.Debug("KargoInstance secret hash changed; forcing re-Apply",
				"previous", getSecretHash(mg), "current", sec.Hash())
			upToDate = false
		}
	}
	if !upToDate {
		if obs, err, ok := e.suppressTerminalWrite(ctx, mg); ok {
			return obs, err
		}
	} else {
		e.clearTerminalWrite(ctx, mg)
	}
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(ctx context.Context, mg *v1alpha1.KargoInstance) (managed.ExternalCreation, error) {
	defer base.PropagateObservedGeneration(mg)
	if err := e.apply(ctx, mg); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.Spec.ForProvider.Name)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha1.KargoInstance) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)
	return managed.ExternalUpdate{}, e.apply(ctx, mg)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha1.KargoInstance) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)
	e.ClearTerminalWriteResource(mg, v1alpha1.KargoInstanceGroupVersionKind)

	name := meta.GetExternalName(mg)
	if name == "" {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, e.Client.DeleteKargoInstance(ctx, name)
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// apply is shared by Create and Update.
//
//nolint:gocyclo // apply orchestrates 6 independent subsystems (secrets, configmap, spec, children, repo creds, status writeback); splitting them yields 6 trivial wrappers without clarity gain.
func (e *external) apply(ctx context.Context, mg *v1alpha1.KargoInstance) error {
	if acd := mg.Spec.ForProvider.Kargo.KargoInstanceSpec.AgentCustomizationDefaults; acd != nil {
		if err := crossplanetypes.ValidateKustomizationYAML(acd.Kustomization); err != nil {
			return fmt.Errorf("spec.forProvider.spec.kargoInstanceSpec.agentCustomizationDefaults.kustomization: %w", err)
		}
	}

	workspaceID, err := e.resolveWorkspaceID(ctx, mg)
	if err != nil {
		return err
	}

	sec, err := resolveKargoSecrets(ctx, e.Kube, mg)
	if err != nil {
		return err
	}
	key, err := kargoInstanceTerminalWriteKey(mg, workspaceID, sec.Hash())
	if err != nil {
		return err
	}
	secretPB, err := kubeSecretToPB(sec.Kargo.Data)
	if err != nil {
		return err
	}
	desiredCM := mg.Spec.ForProvider.KargoConfigMap
	cmPB, err := buildKargoConfigMapPB(desiredCM)
	if err != nil {
		return err
	}

	kargoPB, err := specToPB(mg.Spec.ForProvider, sec.DexConfig.Data)
	if err != nil {
		return err
	}
	children, err := splitKargoResources(mg.Spec.ForProvider.Resources)
	if err != nil {
		return err
	}
	repoCredsPB, err := kargoRepoCredsToPB(sec.RepoCredentials)
	if err != nil {
		return err
	}
	req := &kargov1.ApplyKargoInstanceRequest{
		IdType:                idv1.Type_NAME,
		Id:                    mg.Spec.ForProvider.Name,
		WorkspaceId:           workspaceID,
		Kargo:                 kargoPB,
		KargoConfigmap:        cmPB,
		KargoSecret:           secretPB,
		Projects:              children.Projects,
		Warehouses:            children.Warehouses,
		Stages:                children.Stages,
		AnalysisTemplates:     children.AnalysisTemplates,
		PromotionTasks:        children.PromotionTasks,
		ClusterPromotionTasks: children.ClusterPromotionTasks,
		RepoCredentials:       repoCredsPB,
	}
	if err := e.Client.ApplyKargoInstance(ctx, req); err != nil {
		return e.RecordTerminalWrite(key, reason.ClassifyApplyError(err))
	}
	e.ClearTerminalWrite(key)
	setSecretHash(mg, sec.Hash())
	return nil
}

func (e *external) suppressTerminalWrite(ctx context.Context, mg *v1alpha1.KargoInstance) (managed.ExternalObservation, error, bool) {
	if e.TerminalWrites == nil {
		return managed.ExternalObservation{}, nil, false
	}
	workspaceID, err := e.resolveWorkspaceID(ctx, mg)
	if err != nil {
		return e.SkipTerminalWriteGuard(err)
	}
	sec, err := resolveKargoSecrets(ctx, e.Kube, mg)
	if err != nil {
		return e.SkipTerminalWriteGuard(err)
	}
	key, err := kargoInstanceTerminalWriteKey(mg, workspaceID, sec.Hash())
	if err != nil {
		return e.SkipTerminalWriteGuard(err)
	}
	return e.SuppressTerminalWrite(mg, key)
}

func kargoInstanceTerminalWriteKey(mg *v1alpha1.KargoInstance, workspaceID, secretHash string) (base.TerminalWriteKey, error) {
	return base.NewTerminalWriteKey(mg, v1alpha1.KargoInstanceGroupVersionKind, workspaceID, mg.Spec.ForProvider, secretHash)
}

func (e *external) clearTerminalWrite(ctx context.Context, mg *v1alpha1.KargoInstance) {
	if !e.HasTerminalWriteResource(mg, v1alpha1.KargoInstanceGroupVersionKind) {
		return
	}
	workspaceID, err := e.resolveWorkspaceID(ctx, mg)
	if err != nil {
		e.LogTerminalWriteGuardSkipped(err)
		return
	}
	sec, err := resolveKargoSecrets(ctx, e.Kube, mg)
	if err != nil {
		e.LogTerminalWriteGuardSkipped(err)
		return
	}
	key, err := kargoInstanceTerminalWriteKey(mg, workspaceID, sec.Hash())
	if err != nil {
		e.LogTerminalWriteGuardSkipped(err)
		return
	}
	e.ClearTerminalWrite(key)
}

// resolveWorkspaceID returns the canonical Akuity workspace ID for the
// MR, resolving and caching the organization's default workspace when
// neither the spec nor a previously-stamped status field carries an
// explicit value. The Akuity gateway's KargoInstance HTTP routes are
// all workspace-scoped (`/orgs/{org}/workspaces/{workspace_id}/kargo/instances/...`)
// and template the segment straight into the path; an empty ID
// produces a 404 on Apply, Export, and Delete until a workspace is
// provided or resolved. Before this resolve-and-cache path, first-create
// for a KargoInstance that omitted spec.workspace hot-looped portal-server
// at roughly 350 wasted writes in 12 minutes.
//
// Resolution order:
//  1. spec.forProvider.workspace (user-pinned ID or name; canonical ID
//     short-circuits against status.atProvider.workspace)
//  2. status.atProvider.workspace (controller-cached canonical ID, when spec is empty)
//  3. organization default discovered via the org gateway's ListWorkspaces
//
// On (1) and (3) the resolved canonical ID is stamped on
// status.atProvider.workspace so subsequent reconciles short-circuit
// when the spec is empty. The function preserves spec.forProvider.workspace
// verbatim; the spec carries the user's intent (ID, name, or empty) and
// must not be rewritten by Observe.
func (e *external) resolveWorkspaceID(ctx context.Context, mg *v1alpha1.KargoInstance) (string, error) {
	if ref := mg.Spec.ForProvider.Workspace; ref != "" {
		if ref == mg.Status.AtProvider.Workspace {
			return mg.Status.AtProvider.Workspace, nil
		}
		w, err := e.Client.ResolveWorkspace(ctx, ref)
		if err != nil {
			if reason.IsNotFound(err) {
				return "", reason.AsTerminal(fmt.Errorf("spec.forProvider.workspace %q: %w", ref, err))
			}
			return "", err
		}
		mg.Status.AtProvider.Workspace = w.GetId()
		return w.GetId(), nil
	}
	if id := mg.Status.AtProvider.Workspace; id != "" {
		return id, nil
	}
	w, err := e.Client.ResolveWorkspace(ctx, "")
	if err != nil {
		return "", err
	}
	mg.Status.AtProvider.Workspace = w.GetId()
	return w.GetId(), nil
}

// kargoChildren is the per-kind breakdown of the user's
// spec.forProvider.resources bundle, already marshalled into the
// structpb.Struct shape the ApplyKargoInstance proto expects.
// Repo-credential Secrets intentionally live in
// KargoRepoCredentialSecretRefs (typed refs) rather than this bundle
// so plaintext stays out of the MR spec.
type kargoChildren struct {
	Projects              []*structpb.Struct
	Warehouses            []*structpb.Struct
	Stages                []*structpb.Struct
	AnalysisTemplates     []*structpb.Struct
	PromotionTasks        []*structpb.Struct
	ClusterPromotionTasks []*structpb.Struct
}

// kargoConfigMapUpToDate reports whether every key/value in the
// user's spec.forProvider.kargoConfigMap is present in the gateway's
// current kargo-cm, with subset semantics: extra server-side keys do
// not fire drift (an operator may have added keys out-of-band via the
// Akuity UI or other tooling). A missing key or divergent value does
// fire drift so the next Apply can self-heal.
//
// When the Export response carries no kargo_configmap struct or omits
// the data sub-tree, defer drift instead of reporting every desired
// key as missing. Some gateway builds skip the field entirely on
// freshly-Applied instances; subset comparison would otherwise treat
// the absence as a clear and re-fire ApplyKargoInstance every poll
// indefinitely. The next reconcile retries Export, and a real spec
// edit rotates the generation so the comparator runs again with
// whatever the gateway eventually returns. Returns (ok, observed,
// err) so callers can log the observed shape alongside desired.
func kargoConfigMapUpToDate(desired map[string]string, exp *kargov1.ExportKargoInstanceResponse) (bool, map[string]string, error) {
	if len(desired) == 0 {
		return true, nil, nil
	}
	observed, present, err := extractKargoConfigMapData(exp.GetKargoConfigmap())
	if err != nil {
		return false, nil, fmt.Errorf("kargoConfigMap: %w", err)
	}
	if !present {
		return true, nil, nil
	}
	for k, v := range desired {
		got, ok := observed[k]
		if !ok || got != v {
			return false, observed, nil
		}
	}
	return true, observed, nil
}

// extractKargoConfigMapData pulls the `data` map out of the wrapped
// ConfigMap struct the gateway returns. The second return value
// (`present`) reports whether the gateway response actually carried a
// kargo_configmap.data sub-tree; callers use it to distinguish "Export
// did not surface the CM" (defer drift) from "Export carried a CM with
// zero keys" (drift if desired has any key).
func extractKargoConfigMapData(pb *structpb.Struct) (map[string]string, bool, error) {
	if pb == nil {
		return nil, false, nil
	}
	m := pb.AsMap()
	raw, ok := m["data"]
	if !ok {
		return nil, false, nil
	}
	if raw == nil {
		return map[string]string{}, true, nil
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, false, fmt.Errorf("unexpected kargo_configmap.data shape: %T", raw)
	}
	out := make(map[string]string, len(obj))
	for k, v := range obj {
		switch typed := v.(type) {
		case string:
			out[k] = typed
		case bool:
			out[k] = strconv.FormatBool(typed)
		case nil:
			// Keep an explicit null as an observed empty value. That
			// still differs from any desired non-empty string, so a
			// platform-side clear self-heals instead of being mistaken
			// for a missing/unmanaged key.
			out[k] = ""
		default:
			return nil, false, fmt.Errorf("kargo_configmap.data[%q]: want string, got %T", k, v)
		}
	}
	return out, true, nil
}

// kargoResourcesUpToDate reports whether every declarative Kargo
// child listed in spec.forProvider.resources is present on the
// gateway with an equivalent payload. Mirrors argocdResourcesUpToDate
// on the Instance controller: every desired child must exist in
// observed state.
// Removal from spec does not trigger server-side deletion; operators
// must use the Akuity platform UI or API for that.
func kargoResourcesUpToDate(desired []runtime.RawExtension, exp *kargov1.ExportKargoInstanceResponse) (bool, children.DriftReport, error) {
	if len(desired) == 0 {
		return true, children.DriftReport{}, nil
	}
	desiredIdx, err := children.Index(desired)
	if err != nil {
		return false, children.DriftReport{}, fmt.Errorf("resources: %w", err)
	}
	// Every ApplyKargoInstance-supported collection gets folded into
	// a single observed index. Identity carries apiVersion+kind so
	// cross-kind name collisions are impossible.
	observedAll := make(map[children.Identity]map[string]interface{})
	groups := [][]*structpb.Struct{
		exp.GetProjects(),
		exp.GetWarehouses(),
		exp.GetStages(),
		exp.GetAnalysisTemplates(),
		exp.GetPromotionTasks(),
		exp.GetClusterPromotionTasks(),
		exp.GetServiceAccounts(),
		exp.GetRoles(),
		exp.GetRoleBindings(),
		exp.GetConfigmaps(),
		exp.GetProjectConfigs(),
		exp.GetMessageChannels(),
		exp.GetClusterMessageChannels(),
		exp.GetEventRouters(),
	}
	for _, group := range groups {
		group := group
		idx, err := children.IndexStructs(group)
		if err != nil {
			// Defer the failure to the Apply path rather than failing
			// the reconcile loop on a transient decode issue.
			//nolint:nilerr // Defer transient decode failures to Apply.
			return true, children.DriftReport{}, nil
		}
		for k, v := range idx {
			observedAll[k] = v
		}
	}
	report := children.Compare(desiredIdx, observedAll)
	return report.Empty(), report, nil
}

// splitKargoResources validates each spec.forProvider.resources
// entry and routes it into kargoChildren by (apiVersion, kind). Empty
// input yields a zero struct and no error so callers can compose the
// result without pre-checks.
//
//nolint:gocyclo // The case ladder is one branch per allowed Kargo kind; using a dispatch map would hide the allowlist.
func splitKargoResources(in []runtime.RawExtension) (kargoChildren, error) {
	out := kargoChildren{}
	if len(in) == 0 {
		return out, nil
	}
	for i, raw := range in {
		if len(raw.Raw) == 0 {
			return out, fmt.Errorf("resources[%d]: empty payload", i)
		}
		obj := map[string]interface{}{}
		if err := json.Unmarshal(raw.Raw, &obj); err != nil {
			return out, fmt.Errorf("resources[%d]: invalid JSON: %w", i, err)
		}
		apiVersion, _ := obj["apiVersion"].(string)
		kind, _ := obj["kind"].(string)
		pb, err := structpb.NewStruct(obj)
		if err != nil {
			return out, fmt.Errorf("resources[%d]: structpb encode: %w", i, err)
		}
		switch {
		case apiVersion == kargoAPIVersion && kind == kargoKindProject:
			out.Projects = append(out.Projects, pb)
		case apiVersion == kargoAPIVersion && kind == kargoKindWarehouse:
			out.Warehouses = append(out.Warehouses, pb)
		case apiVersion == kargoAPIVersion && kind == kargoKindStage:
			out.Stages = append(out.Stages, pb)
		case apiVersion == kargoAPIVersion && kind == kargoKindAnalysisTemplate:
			out.AnalysisTemplates = append(out.AnalysisTemplates, pb)
		case apiVersion == kargoAPIVersion && kind == kargoKindPromotionTask:
			out.PromotionTasks = append(out.PromotionTasks, pb)
		case apiVersion == kargoAPIVersion && kind == kargoKindClusterPromotionTsk:
			out.ClusterPromotionTasks = append(out.ClusterPromotionTasks, pb)
		case apiVersion == coreAPIVersion && kind == coreKindSecret:
			return out, fmt.Errorf(errSecretInKargoResources, i)
		default:
			return out, fmt.Errorf("resources[%d]: unsupported %s/%s", i, apiVersion, kind)
		}
	}
	return out, nil
}

// kargoResolvedSecret pairs a referenced kube Secret namespace/name
// with its resolved key/value data. Carrying the source identity into
// the digest means a user who moves or renames the reference with
// identical content still gets a drift signal.
type kargoResolvedSecret struct {
	Namespace string
	Name      string
	Data      map[string]string
}

// kargoResolvedRepoCred is a single entry out of
// KargoRepoCredentialSecretRefs after its backing kube Secret has
// been resolved. Slot+ProjectNamespace form the identity; CredType is
// either explicit or derived from the source Secret label and becomes
// the kargo.akuity.io/cred-type label; SecretNamespace and SecretName
// participate in the hash so a move/rename with identical content
// rotates the digest.
type kargoResolvedRepoCred struct {
	Slot             string
	ProjectNamespace string
	CredType         string
	SecretNamespace  string
	SecretName       string
	Data             map[string]string
}

// resolvedKargoSecrets bundles every referenced Secret the Kargo
// controller reads out of the MR spec. Keeping them together lets
// Observe and Apply share a single hash for rotation detection.
type resolvedKargoSecrets struct {
	// Kargo is the data of the Secret named by
	// spec.forProvider.kargoSecretRef, forwarded to the gateway as
	// the kargo-secret payload.
	Kargo kargoResolvedSecret
	// DexConfig is the data of the Secret named by
	// spec.forProvider.dexConfigSecretRef. It is injected into the
	// wire KargoOidcConfig.DexConfigSecret map as {value: "..."}
	// entries before submission. When the deprecated inline
	// spec.oidcConfig.dexConfigSecret is used instead, DexConfig
	// stays zero and the generated converter handles the wire shape
	// directly.
	DexConfig kargoResolvedSecret
	// RepoCredentials is the resolved form of every
	// spec.forProvider.kargoRepoCredentialSecretRefs entry. The
	// controller synthesizes one labeled Kargo Secret per entry
	// before ApplyKargoInstance.
	RepoCredentials []kargoResolvedRepoCred
}

// Hash combines the digests of every resolved Secret, including the
// referenced Secret namespaces/names, so a move/rename with identical
// content rotates the digest as well as a straight content rotation.
// Repo-credential rotation flows through this hash because the Kargo
// gateway does not round-trip repo_credentials on Export.
func (r resolvedKargoSecrets) Hash() string {
	kh := kargoHashOne(r.Kargo)
	dh := kargoHashOne(r.DexConfig)
	rh := kargoHashRepoCreds(r.RepoCredentials)
	if kh == "" && dh == "" && rh == "" {
		return ""
	}
	return secrets.Hash(map[string]string{"kargo": kh, "dex": dh, "repoCreds": rh})
}

func kargoHashOne(s kargoResolvedSecret) string {
	if s.Namespace == "" && s.Name == "" && len(s.Data) == 0 {
		return ""
	}
	return secrets.Hash(map[string]string{
		"__namespace__": s.Namespace,
		"__name__":      s.Name,
		"__data__":      secrets.Hash(s.Data),
	})
}

// kargoHashRepoCreds mixes every identity bit (slot, project ns,
// cred type, backing Secret namespace/name) and the resolved payload
// into a stable digest. Rotation, source moves/renames, or moving to a
// different Kargo project all rotate the digest.
func kargoHashRepoCreds(entries []kargoResolvedRepoCred) string {
	if len(entries) == 0 {
		return ""
	}
	per := make(map[string]string, len(entries))
	for _, e := range entries {
		key := e.ProjectNamespace + "/" + e.Slot
		per[key] = secrets.Hash(map[string]string{
			"__slot__":       e.Slot,
			"__projectNs__":  e.ProjectNamespace,
			"__credType__":   e.CredType,
			"__secretNs__":   e.SecretNamespace,
			"__secretName__": e.SecretName,
			"__data__":       secrets.Hash(e.Data),
		})
	}
	return secrets.Hash(per)
}

// resolveKargoSecrets loads every Secret referenced by the KargoInstance
// spec (kargo-secret + dex config + repo credentials).
func resolveKargoSecrets(ctx context.Context, kube client.Client, mg *v1alpha1.KargoInstance) (resolvedKargoSecrets, error) {
	out := resolvedKargoSecrets{}
	if ref := mg.Spec.ForProvider.KargoSecretRef; ref != nil {
		resolved, kerr := secrets.Resolve(ctx, kube, ref)
		if kerr != nil {
			return out, secrets.AsTerminalIfConfig(fmt.Errorf("kargoSecretRef: %w", kerr))
		}
		out.Kargo = kargoResolvedSecret{Namespace: resolved.Namespace, Name: resolved.Name, Data: resolved.Data}
	}
	if dex, derr := resolveKargoDexSecret(ctx, kube, mg); derr != nil {
		return out, secrets.AsTerminalIfConfig(derr)
	} else if dex != nil {
		out.DexConfig = *dex
	}
	if refs := mg.Spec.ForProvider.KargoRepoCredentialSecretRefs; len(refs) > 0 {
		creds, rerr := resolveKargoRepoCreds(ctx, kube, refs)
		if rerr != nil {
			return out, secrets.AsTerminalIfConfig(fmt.Errorf("kargoRepoCredentialSecretRefs: %w", rerr))
		}
		out.RepoCredentials = creds
	}
	return out, nil
}

// resolveKargoDexSecret resolves the OIDC dex config Secret when the CR
// references one. Returns nil when the CR carries no dexConfigSecretRef.
func resolveKargoDexSecret(ctx context.Context, kube client.Client, mg *v1alpha1.KargoInstance) (*kargoResolvedSecret, error) {
	oidc := mg.Spec.ForProvider.Kargo.OidcConfig
	if oidc == nil || oidc.DexConfigSecretRef == nil {
		return nil, nil
	}
	resolved, err := secrets.Resolve(ctx, kube, oidc.DexConfigSecretRef)
	if err != nil {
		return nil, fmt.Errorf("spec.oidcConfig.dexConfigSecretRef: %w", err)
	}
	return &kargoResolvedSecret{Namespace: resolved.Namespace, Name: resolved.Name, Data: resolved.Data}, nil
}

// resolveKargoRepoCreds resolves each typed repo-credential ref in
// spec order. ProjectNamespace defaults to SecretRef.Namespace and
// CredType defaults to the source Secret's kargo.akuity.io/cred-type
// label. Duplicate (effective projectNamespace, effective name) pairs
// are rejected at reconcile time, so in-cluster drift of already-
// admitted objects still gets caught.
func resolveKargoRepoCreds(ctx context.Context, kube client.Client, refs []v1alpha1.KargoRepoCredentialSecretRef) ([]kargoResolvedRepoCred, error) {
	out := make([]kargoResolvedRepoCred, 0, len(refs))
	seen := map[string]struct{}{}
	for i := range refs {
		r := &refs[i]
		slot := r.CredentialName()
		if !kargoDNS1123LabelRE.MatchString(slot) {
			return nil, fmt.Errorf("%w: slot %q must match %s", secrets.ErrInvalidSecretReference, slot, kargoDNS1123LabelPattern)
		}
		// Resolve before the final duplicate check because the effective
		// credType may be derived from the source Secret label.
		resolved, err := secrets.Resolve(ctx, kube, &r.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("slot %q: %w", slot, err)
		}
		projectNamespace := effectiveKargoProjectNamespace(r)
		if !kargoDNS1123LabelRE.MatchString(projectNamespace) {
			return nil, fmt.Errorf("%w: slot %q projectNamespace %q must match %s", secrets.ErrInvalidSecretReference, slot, projectNamespace, kargoDNS1123LabelPattern)
		}
		credType := effectiveKargoCredType(r, resolved.Labels)
		if !isKargoCredType(credType) {
			return nil, fmt.Errorf("%w: repo credential %q credType must be one of %s", secrets.ErrInvalidSecretReference, projectNamespace+"/"+slot, kargoCredTypesMsg)
		}
		key := projectNamespace + "/" + slot
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("%w: duplicate slot %q", secrets.ErrInvalidSecretReference, key)
		}
		seen[key] = struct{}{}
		out = append(out, kargoResolvedRepoCred{
			Slot:             slot,
			ProjectNamespace: projectNamespace,
			CredType:         credType,
			SecretNamespace:  resolved.Namespace,
			SecretName:       resolved.Name,
			Data:             resolved.Data,
		})
	}
	return out, nil
}

func effectiveKargoProjectNamespace(r *v1alpha1.KargoRepoCredentialSecretRef) string {
	if r.ProjectNamespace != "" {
		return r.ProjectNamespace
	}
	return r.SecretRef.Namespace
}

func effectiveKargoCredType(r *v1alpha1.KargoRepoCredentialSecretRef, labels map[string]string) string {
	if r.CredType != "" {
		return r.CredType
	}
	return labels[kargoCredTypeLabel]
}

func isKargoCredType(v string) bool {
	return slices.Contains(kargoCredTypes, v)
}

// kargoRepoCredsToPB serializes each resolved repo credential into a
// Kubernetes Secret structpb.Struct, labeled with
// kargo.akuity.io/cred-type and stamped with the target Kargo project
// namespace. Sort order is (projectNamespace, slot) so the Apply
// payload is byte-identical across reconciles for the same input.
// Returns nil on empty input so callers can compose without pre-checks.
func kargoRepoCredsToPB(creds []kargoResolvedRepoCred) ([]*structpb.Struct, error) {
	if len(creds) == 0 {
		return nil, nil
	}
	sorted := append([]kargoResolvedRepoCred(nil), creds...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ProjectNamespace != sorted[j].ProjectNamespace {
			return sorted[i].ProjectNamespace < sorted[j].ProjectNamespace
		}
		return sorted[i].Slot < sorted[j].Slot
	})
	out := make([]*structpb.Struct, 0, len(sorted))
	for _, c := range sorted {
		if len(c.Data) == 0 {
			continue
		}
		byt := make(map[string][]byte, len(c.Data))
		for k, v := range c.Data {
			byt[k] = []byte(v)
		}
		sec := corev1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.Slot,
				Namespace: c.ProjectNamespace,
				Labels:    map[string]string{kargoCredTypeLabel: c.CredType},
			},
			Data: byt,
		}
		pb, err := marshal.APIModelToPBStruct(sec)
		if err != nil {
			return nil, fmt.Errorf("repo credential %s/%s: marshal: %w", c.ProjectNamespace, c.Slot, err)
		}
		out = append(out, pb)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// kubeSecretToPB marshals a resolved secret map into the Kubernetes
// Secret named kargoSecretKey, encoded as a structpb.Struct suitable
// for the Apply payload. Returns nil with no error when data is empty.
func kubeSecretToPB(data map[string]string) (*structpb.Struct, error) {
	if len(data) == 0 {
		return nil, nil
	}
	byt := make(map[string][]byte, len(data))
	for k, v := range data {
		byt[k] = []byte(v)
	}
	sec := corev1.Secret{
		TypeMeta:   metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: kargoSecretKey},
		Data:       byt,
	}
	pb, err := marshal.APIModelToPBStruct(sec)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s secret to protobuf: %w", kargoSecretKey, err)
	}
	return pb, nil
}

// setSecretHash records the SHA256 of every resolved Secret on the
// KargoInstance's status. managed.Reconciler persists AtProvider after
// Create/Update, so the value survives the reconcile round-trip and is
// available for the next Observe to detect rotation. See instance
// controller for the full rationale.
func setSecretHash(mg *v1alpha1.KargoInstance, h string) {
	mg.Status.AtProvider.SecretHash = h
}

func getSecretHash(mg *v1alpha1.KargoInstance) string {
	return mg.Status.AtProvider.SecretHash
}

// buildKargoConfigMapPB serializes the desired kargo-cm payload. An
// empty desired map means the user has no currently managed ConfigMap
// keys, so the Apply payload omits the ConfigMap and leaves any
// platform-side keys alone.
func buildKargoConfigMapPB(data map[string]string) (*structpb.Struct, error) {
	if len(data) == 0 {
		return nil, nil
	}
	cm := corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: kargoCMKey},
		Data:       data,
	}
	pb, err := marshal.APIModelToPBStruct(cm)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s configmap to protobuf: %w", kargoCMKey, err)
	}
	return pb, nil
}

// specToPB marshals the curated KargoInstance into the
// structpb.Struct shape the Kargo ApplyKargoInstance endpoint expects
// under the "kargo" field. The conversion goes through the generated
// KargoSpec converter then through the JSON to map to structpb bridge
// provided by internal/marshal. resolvedDex, when non-nil, replaces
// the wire DexConfigSecret map that the user can't safely supply
// inline (the gateway wraps each value in {value: "..."}).
func specToPB(in v1alpha1.KargoInstanceParameters, resolvedDex map[string]string) (*structpb.Struct, error) {
	wire := akuitytypes.Kargo{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Kargo",
			APIVersion: "kargo.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: in.Name},
	}
	if s := crossplanetypes.KargoSpecSpecToAPI(&in.Kargo); s != nil {
		wire.Spec = *s
	}
	// DefaultShardAgent is owned by the KargoDefaultShardAgent MR.
	// Strip it from the Apply payload so KargoInstance cannot zero a
	// server-side value that KDSA is managing.
	wire.Spec.KargoInstanceSpec.DefaultShardAgent = ""
	if len(resolvedDex) > 0 {
		if wire.Spec.OidcConfig == nil {
			wire.Spec.OidcConfig = &akuitytypes.KargoOidcConfig{}
		}
		// The ref-based path wins over the inline DexConfigSecret so
		// users migrating from the deprecated field get the new
		// behavior once they populate DexConfigSecretRef. The CRD's
		// CEL rule forbids setting both simultaneously, so this only
		// triggers when the user has opted into the ref path.
		wire.Spec.OidcConfig.DexConfigSecret = crossplanetypes.DexConfigSecretResolvedToAPI(resolvedDex)
	}
	pb, err := marshal.APIModelToPBStruct(wire)
	if err != nil {
		return nil, fmt.Errorf("marshal kargo instance: %w", err)
	}
	return pb, nil
}
