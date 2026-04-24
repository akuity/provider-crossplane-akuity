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

// Package kargoinstance is the KargoInstance controller. It drives
// the Akuity Kargo-plane ApplyKargoInstance endpoint through the
// Kargo service gateway configured on internal/clients/akuity.
package kargoinstance

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

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
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/event"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base/children"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
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
// apiVersion / kind pairs; anything else is rejected at reconcile
// entry.
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
)

// driftSpec is the struct-level drift recipe for KargoInstance.
// Resources, KargoConfigMap, and KargoRepoCredentialSecretRefs are
// intentionally ignored here: each has additive semantics, hash-based
// rotation, or TTL re-apply windows that the Observe path handles
// separately. EquateEmpty is contributed by the shared DriftSpec
// baseline.
//
// Normalize absorbs server-echoed fields the user hasn't pinned so the
// first-poll delta doesn't flap: Workspace is set by the user on spec
// but the Kargo Export/Get response doesn't carry workspace back; and
// server-defaulted Subdomain + AkuityIntelligenceExtension always come
// back populated even when the CR omits them. Copying desired→observed
// (or observed→desired for server-defaults) makes the drift compare
// neutral until the user actually changes a pinned field.
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
			// CR round-trips user order. Without order-insensitive
			// compare, a user writing `[groups, email]` and a server
			// echoing `[email, groups]` hot-loop Apply on every poll.
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
			// forever. User SET / FLIP still propagates because desired
			// non-nil short-circuits this branch — terraform parity is
			// resource_akp_kargo_schema.go:187 where the attribute is
			// Computed and TF carries last-known-state as effective plan.
			if desired.Kargo.KargoInstanceSpec.GcConfig == nil {
				desired.Kargo.KargoInstanceSpec.GcConfig =
					observed.Kargo.KargoInstanceSpec.GcConfig
			}
			// DefaultShardAgent is owned by the KargoDefaultShardAgent MR,
			// not the KargoInstance MR. Mirror terraform's
			// resource_akp_kargo.go:352 pattern (`delete(kargoInstanceSpec,
			// "defaultShardAgent")` in the Apply payload builder) by
			// treating an unset desired as "leave alone"; without this
			// KargoInstance fights KDSA on every poll because KDSA writes
			// the agent ID server-side and KargoInstance's desired is "".
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

// repoCredsReapplyTTL is the upper bound between two successful
// Applies of spec.forProvider.kargoRepoCredentialSecretRefs. Because
// the Kargo Export response does not return repo_credentials, an
// out-of-band deletion via the Akuity UI would otherwise stay
// invisible to Observe (local SecretHash unchanged → UpToDate=true
// forever). Forcing a re-Apply past this TTL self-heals that case at
// the cost of one Apply per hour per instance carrying repo creds.
const repoCredsReapplyTTL = 1 * time.Hour

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

//nolint:gocyclo // Observe coordinates struct cmp + 2 export-based drift checks + secret hash + repo-cred TTL; the linear branching is the simplest readable form.
func (e *external) Observe(ctx context.Context, mg *v1alpha1.KargoInstance) (managed.ExternalObservation, error) {
	defer base.PropagateObservedGeneration(mg)
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

	actual, err := apiToSpec(ki)
	if err != nil {
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	// KargoSecretRef is spec-only (the gateway doesn't round-trip
	// it); carry it forward so the struct comparison doesn't flag it
	// as drift. The SecretHash in AtProvider catches rotation.
	//
	// KargoConfigMap participates in drift independently below,
	// against the ExportKargoInstance response, so we do NOT carry
	// desired into actual — doing so would mask real drift (a cleared
	// key or out-of-band edit).
	//
	// KargoResources drift is computed independently below against
	// the ExportKargoInstance response; we exclude it from the
	// struct-level comparison via cmpopts.IgnoreFields instead of
	// carrying it over, so real drift can surface.
	actual.KargoSecretRef = mg.Spec.ForProvider.KargoSecretRef

	// DexConfigSecretRef lives nested under spec.oidcConfig and is
	// spec-only; carry it forward so the struct comparison doesn't
	// flag it as drift. Inline spec.oidcConfig.dexConfigSecret values
	// DO round-trip through apiToSpec, so they stay under the control
	// of the comparator.
	if desired := mg.Spec.ForProvider.Kargo.OidcConfig; desired != nil && desired.DexConfigSecretRef != nil {
		if actual.Kargo.OidcConfig == nil {
			actual.Kargo.OidcConfig = &crossplanetypes.KargoOidcConfig{}
		}
		actual.Kargo.OidcConfig.DexConfigSecretRef = desired.DexConfigSecretRef
	}

	// Preserve controller-authored AtProvider fields across the refresh.
	// apiToObservation rebuilds the struct from gateway data, which has
	// no SecretHash / ConfigMapHash / RepoCredsAppliedAt; without
	// this carry-over every Observe would zero them, masking rotation
	// and the CM-tombstone state until the next Create/Update writes
	// new values.
	prevSecretHash := mg.Status.AtProvider.SecretHash
	prevCMHashCarry := mg.Status.AtProvider.ConfigMapHash
	prevRepoCredsAt := mg.Status.AtProvider.RepoCredsAppliedAt
	mg.Status.AtProvider = observation.KargoInstance(ki)
	// Mirror the observed Kargo sub-tree onto AtProvider so
	// compositions and dashboards can read the effective server-side
	// spec without a separate Export call. Parity with
	// InstanceObservation.ArgoCD. The observation projector can't
	// build this itself because reassembling it requires the full
	// apiToSpec transform (structpb → wire → Crossplane spec) which
	// lives in the kargoinstance package, not in the observation
	// package (circular import otherwise).
	mg.Status.AtProvider.Kargo = actual.Kargo
	mg.Status.AtProvider.SecretHash = prevSecretHash
	mg.Status.AtProvider.ConfigMapHash = prevCMHashCarry
	mg.Status.AtProvider.RepoCredsAppliedAt = prevRepoCredsAt
	base.SetHealthCondition(mg, mg.Status.AtProvider.HealthStatus.Code == 1)

	// Struct-level comparison of the primary spec shape. Resources,
	// KargoConfigMap, and KargoRepoCredentialSecretRefs are checked
	// separately below — they have additive semantics, hash-based
	// rotation, or TTL re-apply windows that a plain struct cmp
	// cannot express. The shared DriftSpec also contributes
	// utilcmp.EquateEmpty() so nil-vs-empty sub-trees resolve equal.
	structSpec := driftSpec()
	desired := mg.Spec.ForProvider
	upToDate, err := base.EvaluateDrift(ctx, structSpec, &desired, &actual, e.Logger, "KargoInstance")
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	// kargoConfigMap drift: combine a local hash check with the
	// Export-based subset check.
	//   - hash mismatch (desired-now vs last-applied) catches key
	//     removals and value changes without a gateway round-trip.
	//     This also handles the tombstone case: desired empty +
	//     prev hash non-empty → re-Apply with an empty payload.
	//   - export subset check catches out-of-band server changes to
	//     keys the user still has in spec.
	// Export must run whenever the MR has ever tracked a CM, not
	// just when the desired map is currently non-empty; otherwise
	// a transient clear would evade detection on the next reconcile.
	prevCMHash := getConfigMapHash(mg)
	desiredCMHash := hashConfigMap(mg.Spec.ForProvider.KargoConfigMap)
	if upToDate && desiredCMHash != prevCMHash {
		e.Logger.Debug("kargoConfigMap hash changed since last apply; forcing re-Apply",
			"previous", prevCMHash, "current", desiredCMHash)
		upToDate = false
	}

	// Export-based drift check runs when the user has spec-level
	// opinions on either kargoConfigMap (current or historical) or
	// kargoResources. A single Export serves both, minimising
	// gateway round-trips.
	needsExport := len(mg.Spec.ForProvider.KargoConfigMap) > 0 ||
		prevCMHash != "" ||
		len(mg.Spec.ForProvider.Resources) > 0
	if upToDate && needsExport {
		exp, err := e.Client.ExportKargoInstance(ctx, meta.GetExternalName(mg), mg.Spec.ForProvider.Workspace)
		if err != nil {
			// A failed Export on an otherwise-healthy instance is
			// recoverable; leaving upToDate=true lets the next poll
			// retry rather than stampede Apply on a transient error.
			e.Logger.Debug("KargoInstance export failed; deferring child-drift check", "err", err)
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
		// Periodic re-apply for repo credentials: the Kargo Export
		// response has no repo_credentials field, so OOB deletions
		// via the Akuity UI cannot be detected. Force a re-Apply
		// past the TTL so the next reconcile resurfaces the credential
		// server-side.
		if upToDate && len(mg.Spec.ForProvider.KargoRepoCredentialSecretRefs) > 0 {
			if ts := mg.Status.AtProvider.RepoCredsAppliedAt; ts == nil || time.Since(ts.Time) > repoCredsReapplyTTL {
				e.Logger.Debug("kargoRepoCredentialSecretRefs TTL elapsed; forcing re-Apply",
					"lastApplied", ts, "ttl", repoCredsReapplyTTL)
				upToDate = false
			}
		}
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

	sec, err := resolveKargoSecrets(ctx, e.Kube, mg)
	if err != nil {
		return err
	}
	secretPB, err := kubeSecretToPB(sec.Kargo.Data)
	if err != nil {
		return err
	}
	// CM tombstone: if the controller has ever applied a non-empty CM
	// (prevHash != "") and the current desired is empty, send an
	// explicit empty ConfigMap payload so the gateway can clear the
	// previously-applied keys. Without the tombstone, emptying the
	// field in spec would be an un-Applyable delete.
	prevCMHash := getConfigMapHash(mg)
	desiredCM := mg.Spec.ForProvider.KargoConfigMap
	cmPB, err := buildKargoConfigMapPB(desiredCM, prevCMHash != "")
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
		WorkspaceId:           mg.Spec.ForProvider.Workspace,
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
		return err
	}
	setSecretHash(mg, sec.Hash())
	setConfigMapHash(mg, hashConfigMap(desiredCM))
	if len(mg.Spec.ForProvider.KargoRepoCredentialSecretRefs) > 0 {
		now := metav1.Now()
		mg.Status.AtProvider.RepoCredsAppliedAt = &now
	} else {
		mg.Status.AtProvider.RepoCredsAppliedAt = nil
	}
	return nil
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
// When the Export response contains no kargo_configmap struct, the
// observed map is treated as empty — any desired keys are reported
// missing. Returns (ok, observed, err) so callers can log the observed
// shape alongside desired.
func kargoConfigMapUpToDate(desired map[string]string, exp *kargov1.ExportKargoInstanceResponse) (bool, map[string]string, error) {
	if len(desired) == 0 {
		return true, nil, nil
	}
	observed, err := extractKargoConfigMapData(exp.GetKargoConfigmap())
	if err != nil {
		return false, nil, fmt.Errorf("kargoConfigMap: %w", err)
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
// ConfigMap struct the gateway returns. Absent / non-object shapes are
// treated as empty data rather than errors so a half-initialised
// instance (no CM yet) still lets drift surface the missing keys.
func extractKargoConfigMapData(pb *structpb.Struct) (map[string]string, error) {
	if pb == nil {
		return nil, nil
	}
	m := pb.AsMap()
	raw, ok := m["data"]
	if !ok {
		return nil, nil
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected kargo_configmap.data shape: %T", raw)
	}
	out := make(map[string]string, len(obj))
	for k, v := range obj {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("kargo_configmap.data[%q]: want string, got %T", k, v)
		}
		out[k] = s
	}
	return out, nil
}

// kargoResourcesUpToDate reports whether every declarative Kargo
// child listed in spec.forProvider.resources is present on the
// gateway with an equivalent payload. Mirrors argocdResourcesUpToDate
// on the Instance controller: additive semantics, desired ⊆ observed.
// Removal from spec does NOT trigger server-side deletion — operators
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
			//nolint:nilerr // intentional swallow; see comment above
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
//nolint:gocyclo // The case ladder is one branch per allowed Kargo kind; refactoring to a map of kind → handler obscures the allowlist this function documents.
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

// kargoResolvedSecret pairs a referenced kube Secret name with its
// resolved key/value data. Carrying the name into the digest means a
// user who renames the reference (from Secret "A" to Secret "B" with
// identical content) still gets a drift signal.
type kargoResolvedSecret struct {
	Name string
	Data map[string]string
}

// kargoResolvedRepoCred is a single entry out of
// KargoRepoCredentialSecretRefs after its backing kube Secret has
// been resolved. Slot+ProjectNamespace form the identity; CredType
// becomes the kargo.akuity.io/cred-type label; SecretName participates
// in the hash so a rename-with-identical-content rotates the digest.
type kargoResolvedRepoCred struct {
	Slot             string
	ProjectNamespace string
	CredType         string
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
	// controller synthesises one labelled Kargo Secret per entry
	// before ApplyKargoInstance.
	RepoCredentials []kargoResolvedRepoCred
}

// Hash combines the digests of every resolved Secret, including the
// referenced Secret names, so a rename-with-identical-content rotates
// the digest as well as a straight content rotation. Repo-credential
// rotation flows through this hash because the Kargo gateway does
// not round-trip repo_credentials on Export.
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
	if s.Name == "" && len(s.Data) == 0 {
		return ""
	}
	return secrets.Hash(map[string]string{
		"__ref__":  s.Name,
		"__data__": secrets.Hash(s.Data),
	})
}

// kargoHashRepoCreds mixes every identity bit (slot, project ns,
// cred type, backing Secret name) and the resolved payload into a
// stable digest. A change in any of those — rotation, rename, moving
// to a different Kargo project — rotates the digest.
func kargoHashRepoCreds(entries []kargoResolvedRepoCred) string {
	if len(entries) == 0 {
		return ""
	}
	per := make(map[string]string, len(entries))
	for _, e := range entries {
		key := e.ProjectNamespace + "/" + e.Slot
		per[key] = secrets.Hash(map[string]string{
			"__slot__":      e.Slot,
			"__projectNs__": e.ProjectNamespace,
			"__credType__":  e.CredType,
			"__secret__":    e.SecretName,
			"__data__":      secrets.Hash(e.Data),
		})
	}
	return secrets.Hash(per)
}

// kargoHasAnySecretRef reports whether the KargoInstance spec declares
// any SecretRef. Lets resolveKargoSecrets skip the PC lookup entirely
// when the user has wired no credentials (the minimal-spec path).
func kargoHasAnySecretRef(mg *v1alpha1.KargoInstance) bool {
	fp := mg.Spec.ForProvider
	if fp.KargoSecretRef != nil {
		return true
	}
	if oidc := fp.Kargo.OidcConfig; oidc != nil && oidc.DexConfigSecretRef != nil {
		return true
	}
	return len(fp.KargoRepoCredentialSecretRefs) > 0
}

// kargoSecretNamespace returns the namespace used to resolve the
// KargoInstance's SecretRefs. KargoInstance is cluster-scoped so
// mg.GetNamespace() is empty; fall back to the ProviderConfig's
// credentialsSecretRef namespace (symmetric with Instance).
func kargoSecretNamespace(ctx context.Context, kube client.Client, mg *v1alpha1.KargoInstance) (string, error) {
	if ns := mg.GetNamespace(); ns != "" {
		return ns, nil
	}
	ref := mg.GetProviderConfigReference()
	if ref == nil {
		return "", fmt.Errorf("cluster-scoped KargoInstance %q has no providerConfigRef; cannot resolve SecretRef namespace", mg.GetName())
	}
	pc := &apisv1alpha1.ProviderConfig{}
	if err := kube.Get(ctx, k8stypes.NamespacedName{Name: ref.Name}, pc); err != nil {
		return "", fmt.Errorf("cannot resolve SecretRef namespace: get ProviderConfig %q: %w", ref.Name, err)
	}
	if ns := pc.Spec.CredentialsSecretRef.Namespace; ns != "" {
		return ns, nil
	}
	return "", fmt.Errorf("ProviderConfig %q has no credentialsSecretRef.namespace to use as SecretRef lookup namespace", ref.Name)
}

// resolveKargoSecrets loads every Secret referenced by the KargoInstance
// spec (kargo-secret + dex config + repo credentials).
func resolveKargoSecrets(ctx context.Context, kube client.Client, mg *v1alpha1.KargoInstance) (resolvedKargoSecrets, error) {
	out := resolvedKargoSecrets{}
	if !kargoHasAnySecretRef(mg) {
		return out, nil
	}
	ns, err := kargoSecretNamespace(ctx, kube, mg)
	if err != nil {
		return out, err
	}
	if ref := mg.Spec.ForProvider.KargoSecretRef; ref != nil {
		kargoData, kerr := secrets.ResolveAllKeys(ctx, kube, ns, ref)
		if kerr != nil {
			return out, fmt.Errorf("kargoSecretRef: %w", kerr)
		}
		out.Kargo = kargoResolvedSecret{Name: ref.Name, Data: kargoData}
	}
	var dexRef *xpv1.LocalSecretReference
	if oidc := mg.Spec.ForProvider.Kargo.OidcConfig; oidc != nil && oidc.DexConfigSecretRef != nil {
		dexRef = &xpv1.LocalSecretReference{Name: oidc.DexConfigSecretRef.Name}
	}
	if dexRef != nil {
		data, derr := secrets.ResolveAllKeys(ctx, kube, ns, dexRef)
		if derr != nil {
			return out, fmt.Errorf("spec.oidcConfig.dexConfigSecretRef: %w", derr)
		}
		out.DexConfig = kargoResolvedSecret{Name: dexRef.Name, Data: data}
	}
	if refs := mg.Spec.ForProvider.KargoRepoCredentialSecretRefs; len(refs) > 0 {
		creds, rerr := resolveKargoRepoCreds(ctx, kube, ns, refs)
		if rerr != nil {
			return out, fmt.Errorf("kargoRepoCredentialSecretRefs: %w", rerr)
		}
		out.RepoCredentials = creds
	}
	return out, nil
}

// resolveKargoRepoCreds resolves each typed repo-credential ref in
// spec order. Duplicate (projectNamespace, name) pairs are rejected
// at reconcile time in addition to the CRD-level CEL uniqueness rule,
// so in-cluster drift of already-admitted objects still gets caught.
func resolveKargoRepoCreds(ctx context.Context, kube client.Client, ns string, refs []v1alpha1.KargoRepoCredentialSecretRef) ([]kargoResolvedRepoCred, error) {
	out := make([]kargoResolvedRepoCred, 0, len(refs))
	seen := map[string]struct{}{}
	for i := range refs {
		r := &refs[i]
		key := r.ProjectNamespace + "/" + r.Name
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("duplicate slot %q", key)
		}
		seen[key] = struct{}{}
		data, err := secrets.ResolveAllKeys(ctx, kube, ns, &r.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("slot %q: %w", key, err)
		}
		out = append(out, kargoResolvedRepoCred{
			Slot:             r.Name,
			ProjectNamespace: r.ProjectNamespace,
			CredType:         r.CredType,
			SecretName:       r.SecretRef.Name,
			Data:             data,
		})
	}
	return out, nil
}

// kargoRepoCredsToPB serialises each resolved repo credential into a
// Kubernetes Secret structpb.Struct, labelled with
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

// setConfigMapHash records the SHA256 of the last-applied
// kargo-cm payload so Observe can spot key removals that the
// subset check on the Export response would otherwise miss.
func setConfigMapHash(mg *v1alpha1.KargoInstance, h string) {
	mg.Status.AtProvider.ConfigMapHash = h
}

func getConfigMapHash(mg *v1alpha1.KargoInstance) string {
	return mg.Status.AtProvider.ConfigMapHash
}

// hashConfigMap returns a stable digest for a map[string]string.
// An empty / nil input yields the empty string so callers can
// distinguish "never applied" (empty prev hash) from "applied then
// cleared" by comparing against the stored value.
func hashConfigMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	return secrets.Hash(m)
}

// buildKargoConfigMapPB serialises the desired kargo-cm payload,
// respecting the tombstone semantics needed for B1: when the user
// previously applied a non-empty map and now wants it cleared, send
// an explicit empty ConfigMap struct so the gateway understands the
// intent as "clear all keys" rather than "no opinion, leave as-is".
// Returns (nil, nil) only when the field has never been set, which
// is the only case the gateway may safely ignore.
func buildKargoConfigMapPB(data map[string]string, tombstoneOnEmpty bool) (*structpb.Struct, error) {
	if len(data) == 0 && !tombstoneOnEmpty {
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
// KargoSpec converter then through the JSON→map→structpb bridge
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
	// DefaultShardAgent is owned by the KargoDefaultShardAgent MR. Mirror
	// terraform's resource_akp_kargo.go:352 pattern and strip the field
	// from the Apply payload so KargoInstance cannot zero a server-side
	// value that KDSA is managing. Drift Observe-side also ignores this
	// field (see driftSpec.Normalize).
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
