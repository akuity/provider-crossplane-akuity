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

package instance

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/event"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
	utilcmp "github.com/akuityio/provider-crossplane-akuity/internal/utils/cmp"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/pointer"
)

const errTransformInstance = "cannot transform Crossplane instance to Akuity API instance"

// Setup adds a controller that reconciles Instance managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.InstanceGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

	conn := &base.Connector[*v1alpha1.Instance]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewLegacyProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha1.Instance] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.InstanceGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha1.Instance](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
		managed.WithManagementPolicies(),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Instance{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha1.Instance) (managed.ExternalObservation, error) { //nolint:gocyclo
	defer base.PropagateObservedGeneration(mg)

	// Short-circuit on a cached terminal write before any gateway
	// round-trip. Without this, Crossplane's NameAsExternalName initializer
	// stamps the external-name annotation up front, so a Create that fails
	// terminally on bad input (e.g. inline v1/Secret in resources[],
	// missing referenced Secret, malformed Kustomization) leaves Observe
	// in the GetInstance branch on every subsequent reconcile: Get returns
	// NotFound, ResourceExists=false, the reconciler fires Create again,
	// BuildApplyInstanceRequest re-rejects, and controller-runtime
	// backoff requeues at ~2s. The terminal-write guard already records
	// the failure but the prior empty-external-name suppress site never
	// fires under the default initializer. Consult the guard up front;
	// the key carries generation and payload fingerprint, so a spec edit
	// or Secret rotation rotates the key and the next reconcile proceeds
	// normally. HasTerminalWriteResource is a cheap map lookup so happy-
	// path Observes only pay the secret resolve when an entry exists.
	if e.HasTerminalWriteResource(mg, v1alpha1.InstanceGroupVersionKind) {
		if obs, err, ok := e.suppressTerminalWrite(ctx, mg); ok {
			return obs, err
		}
	}

	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	akuityInstance, err := e.Client.GetInstance(ctx, meta.GetExternalName(mg))
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

	akuityExportedInstance, err := e.Client.ExportInstance(ctx, meta.GetExternalName(mg))
	if err != nil {
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	actualInstance, err := observation.InstanceSpec(akuityInstance, akuityExportedInstance)
	if err != nil {
		newErr := fmt.Errorf("could not transform instance spec from Akuity API to internal instance spec: %w", err)
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, newErr
	}
	presence := base.ForProviderPresence(ctx, e.Kube, mg, v1alpha1.InstanceGroupVersionKind)

	if err := lateInitializeInstance(&mg.Spec.ForProvider, akuityInstance, akuityExportedInstance); err != nil {
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	instanceObservation, err := observation.Instance(akuityInstance, akuityExportedInstance)
	if err != nil {
		newErr := fmt.Errorf("could not transform instance from Akuity API to Crossplane instance observation: %w", err)
		mg.SetConditions(xpv1.ReconcileError(newErr))
		return managed.ExternalObservation{}, newErr
	}

	// SecretHash is written by Create/Update after a successful Apply and
	// drives rotation drift in the block below. instanceObservation is
	// projected from the gateway response (which returns secret data
	// masked/nil) and therefore carries no SecretHash. Assigning the
	// whole struct would clobber the controller-managed hash every poll
	// and re-trigger Apply on every reconcile.
	// Preserve across the assignment.
	preservedSecretHash := mg.Status.AtProvider.SecretHash
	mg.Status.AtProvider = instanceObservation
	mg.Status.AtProvider.SecretHash = preservedSecretHash
	base.SetHealthCondition(mg, instanceObservation.HealthStatus.Code == 1)

	// DeepCopy so Normalize's map mutations (ArgoCDConfigMap rewrites,
	// ignored-key deletions) don't leak back into the managed resource.
	spec := driftSpec()
	spec.Presence = presence
	desired := mg.Spec.ForProvider.DeepCopy()
	observed := actualInstance.Spec.ForProvider.DeepCopy()
	isUpToDate, err := base.EvaluateDrift(ctx, spec, desired, observed, e.Logger, "Instance")
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	// Secret rotation drift: SecretRef fields are ignored by the struct
	// compare (drift spec IgnoreFields) because the Export response
	// returns the Secret data masked/nil. When the rest of the struct
	// is up-to-date, re-resolve every referenced Secret and compare the
	// digest against the last-applied hash. A difference means the user
	// rotated one of the Secrets; we flip UpToDate=false so the reconciler
	// fires Update, which re-resolves, re-Applies with the new payload,
	// and refreshes SecretHash.
	if isUpToDate {
		sec, serr := resolveInstanceSecrets(ctx, e.Kube, mg)
		if serr != nil {
			mg.SetConditions(xpv1.ReconcileError(serr))
			return managed.ExternalObservation{}, serr
		}
		if sec.Hash() != mg.Status.AtProvider.SecretHash {
			e.Logger.Debug("Instance secret hash changed; forcing re-Apply",
				"previous", mg.Status.AtProvider.SecretHash, "current", sec.Hash())
			isUpToDate = false
		}
	}

	// Declarative Argo CD child-resource drift: Application,
	// ApplicationSet, AppProject manifests on
	// spec.forProvider.resources are additive: every desired child must
	// be present in observed state.
	// The struct compare ignores the Resources slice (Export does not
	// return them inline; they live on the Applications /
	// ApplicationSets / AppProjects slices on the response). Walk the
	// already-fetched Export response and report drift if any desired
	// child is missing or not subset-matched on the gateway. Removing
	// an entry from spec is intentionally not drift; operators must
	// delete via the Akuity platform UI.
	if isUpToDate && len(mg.Spec.ForProvider.Resources) > 0 {
		ok, report, rerr := argocdResourcesUpToDate(mg.Spec.ForProvider.Resources, akuityExportedInstance)
		if rerr != nil {
			mg.SetConditions(xpv1.ReconcileError(rerr))
			return managed.ExternalObservation{}, rerr
		}
		if !ok {
			e.Logger.Debug("argocd resources drift detected",
				"missing", report.Missing, "changed", report.Changed)
			isUpToDate = false
		}
	}

	if !isUpToDate {
		if obs, err, ok := e.suppressTerminalWrite(ctx, mg); ok {
			return obs, err
		}
	} else {
		e.clearTerminalWrite(ctx, mg)
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mg *v1alpha1.Instance) (managed.ExternalCreation, error) {
	defer base.PropagateObservedGeneration(mg)

	sec, err := resolveInstanceSecrets(ctx, e.Kube, mg)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	key, err := instanceTerminalWriteKey(mg, sec)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	request, err := BuildApplyInstanceRequest(*mg, sec)
	if err != nil {
		return managed.ExternalCreation{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(fmt.Errorf("%s: %w", errTransformInstance, err)))
	}

	if err := e.Client.ApplyInstance(ctx, request); err != nil {
		return managed.ExternalCreation{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(err))
	}
	e.ClearTerminalWrite(key)
	mg.Status.AtProvider.SecretHash = sec.Hash()
	meta.SetExternalName(mg, request.GetId())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha1.Instance) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)

	sec, err := resolveInstanceSecrets(ctx, e.Kube, mg)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	key, err := instanceTerminalWriteKey(mg, sec)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	request, err := BuildApplyInstanceRequest(*mg, sec)
	if err != nil {
		return managed.ExternalUpdate{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(fmt.Errorf("%s: %w", errTransformInstance, err)))
	}
	if err := e.Client.ApplyInstance(ctx, request); err != nil {
		return managed.ExternalUpdate{}, e.RecordTerminalWrite(key, reason.ClassifyApplyError(err))
	}
	e.ClearTerminalWrite(key)
	mg.Status.AtProvider.SecretHash = sec.Hash()
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg *v1alpha1.Instance) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)
	e.ClearTerminalWriteResource(mg, v1alpha1.InstanceGroupVersionKind)

	externalName := meta.GetExternalName(mg)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	return managed.ExternalDelete{}, e.Client.DeleteInstance(ctx, externalName)
}

func (e *external) Disconnect(_ context.Context) error { return nil }

func (e *external) suppressTerminalWrite(ctx context.Context, mg *v1alpha1.Instance) (managed.ExternalObservation, error, bool) {
	if e.TerminalWrites == nil {
		return managed.ExternalObservation{}, nil, false
	}
	sec, err := resolveInstanceSecrets(ctx, e.Kube, mg)
	if err != nil {
		return e.SkipTerminalWriteGuard(err)
	}
	key, err := instanceTerminalWriteKey(mg, sec)
	if err != nil {
		return e.SkipTerminalWriteGuard(err)
	}
	return e.SuppressTerminalWrite(mg, key)
}

func instanceTerminalWriteKey(mg *v1alpha1.Instance, sec resolvedInstanceSecrets) (base.TerminalWriteKey, error) {
	return base.NewTerminalWriteKey(mg, v1alpha1.InstanceGroupVersionKind, mg.Spec.ForProvider, sec.Hash())
}

func (e *external) clearTerminalWrite(ctx context.Context, mg *v1alpha1.Instance) {
	if !e.HasTerminalWriteResource(mg, v1alpha1.InstanceGroupVersionKind) {
		return
	}
	sec, err := resolveInstanceSecrets(ctx, e.Kube, mg)
	if err != nil {
		e.LogTerminalWriteGuardSkipped(err)
		return
	}
	key, err := instanceTerminalWriteKey(mg, sec)
	if err != nil {
		e.LogTerminalWriteGuardSkipped(err)
		return
	}
	e.ClearTerminalWrite(key)
}

func lateInitializeInstance(in *v1alpha1.InstanceParameters, instance *argocdv1.Instance, exportedInstance *argocdv1.ExportInstanceResponse) error {
	in.ArgoCD.Spec.InstanceSpec.Subdomain = pointer.LateInitialize(in.ArgoCD.Spec.InstanceSpec.Subdomain, instance.GetSpec().GetSubdomain())
	in.ArgoCD.Spec.InstanceSpec.DeclarativeManagementEnabled = pointer.LateInitialize(in.ArgoCD.Spec.InstanceSpec.DeclarativeManagementEnabled, ptr.To(instance.GetSpec().GetDeclarativeManagementEnabled()))
	in.ArgoCD.Spec.InstanceSpec.AppsetPolicy = pointer.LateInitialize(in.ArgoCD.Spec.InstanceSpec.AppsetPolicy, observation.AppsetPolicy(instance.GetSpec().GetAppsetPolicy()))

	if in.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults == nil || in.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults.Kustomization == "" {
		clusterCustomizationDefaults, err := observation.ClusterCustomization(instance.GetSpec().GetClusterCustomizationDefaults())
		if err != nil {
			return fmt.Errorf("could not late initialize instance cluster customization defaults: %w", err)
		}

		if in.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults == nil {
			in.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults = clusterCustomizationDefaults
		} else {
			in.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults.Kustomization = clusterCustomizationDefaults.Kustomization
		}
	}

	return lateInitializeInstanceConfigMaps(in, exportedInstance)
}

//nolint:gocyclo
func lateInitializeInstanceConfigMaps(in *v1alpha1.InstanceParameters, exportedInstance *argocdv1.ExportInstanceResponse) error {
	if in.ArgoCDConfigMap == nil {
		argocdConfigMap, err := observation.ConfigMapData(observation.ArgocdCMKey, exportedInstance.GetArgocdConfigmap())
		if err != nil {
			return fmt.Errorf("could not late initialize instance configmap %s: %w", observation.ArgocdCMKey, err)
		}
		in.ArgoCDConfigMap = argocdConfigMap
	}

	if in.ArgoCDSSHKnownHostsConfigMap == nil {
		argocdKnownHostsConfigMap, err := observation.ConfigMapData(observation.ArgocdSSHKnownHostsCMKey, exportedInstance.GetArgocdKnownHostsConfigmap())
		if err != nil {
			return fmt.Errorf("could not late initialize instance configmap %s: %w", observation.ArgocdSSHKnownHostsCMKey, err)
		}
		in.ArgoCDSSHKnownHostsConfigMap = argocdKnownHostsConfigMap
	}

	if in.ArgoCDRBACConfigMap == nil {
		argocdRBACConfigMap, err := observation.ConfigMapData(observation.ArgocdRBACCMKey, exportedInstance.GetArgocdRbacConfigmap())
		if err != nil {
			return fmt.Errorf("could not late initialize instance configmap %s: %w", observation.ArgocdRBACCMKey, err)
		}
		in.ArgoCDRBACConfigMap = argocdRBACConfigMap
	}

	if in.ArgoCDTLSCertsConfigMap == nil {
		argocdTLSCertsConfigMap, err := observation.ConfigMapData(observation.ArgocdTLSCertsCMKey, exportedInstance.GetArgocdTlsCertsConfigmap())
		if err != nil {
			return fmt.Errorf("could not late initialize instance configmap %s: %w", observation.ArgocdTLSCertsCMKey, err)
		}
		in.ArgoCDTLSCertsConfigMap = argocdTLSCertsConfigMap
	}

	if in.ConfigManagementPlugins == nil {
		configManagementPlugins, err := observation.ConfigManagementPlugins(exportedInstance.GetConfigManagementPlugins())
		if err != nil {
			return fmt.Errorf("could not late initialize instance config management plugins: %w", err)
		}
		in.ConfigManagementPlugins = configManagementPlugins
	}

	return nil
}

type ResourceCustomization struct {
	Group             string `yaml:"-"`
	Kind              string `yaml:"-"`
	Health            string `yaml:"health.lua,omitempty"`
	Actions           string `yaml:"actions,omitempty"`
	IgnoreDifferences string `yaml:"ignoreDifferences,omitempty"`
	KnownTypeFields   string `yaml:"knownTypeFields,omitempty"`
}

// normalizeInstanceParameters adopts API defaults and canonical forms
// that should not be treated as user-visible drift.
func normalizeInstanceParameters(managedInstance, actualInstance *v1alpha1.InstanceParameters) { //nolint:gocyclo
	if managedInstance == nil || actualInstance == nil {
		return
	}
	// Workspace is a routing selector. The gateway reports the canonical ID,
	// while spec may be empty, an ID, or a name; keep the drift comparison
	// neutral and let the client resolve it for workspace-scoped calls.
	actualInstance.Workspace = managedInstance.Workspace

	if managedInstance.ArgoCD != nil {
		// The platform may default MultiClusterK8SDashboardEnabled
		// even when the CR omits it.
		if managedInstance.ArgoCD.Spec.InstanceSpec.MultiClusterK8SDashboardEnabled == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.MultiClusterK8SDashboardEnabled = actualInstance.ArgoCD.Spec.InstanceSpec.MultiClusterK8SDashboardEnabled
		}

		// Only one of Fqdn and Subdomain should be set. If both are
		// present, adopt the platform's canonical pair.
		if managedInstance.ArgoCD.Spec.InstanceSpec.Fqdn != "" && managedInstance.ArgoCD.Spec.InstanceSpec.Subdomain != "" {
			managedInstance.ArgoCD.Spec.InstanceSpec.Subdomain = actualInstance.ArgoCD.Spec.InstanceSpec.Subdomain
			managedInstance.ArgoCD.Spec.InstanceSpec.Fqdn = actualInstance.ArgoCD.Spec.InstanceSpec.Fqdn
		}

		if managedInstance.ArgoCD.Spec.InstanceSpec.KubeVisionConfig == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.KubeVisionConfig = actualInstance.ArgoCD.Spec.InstanceSpec.KubeVisionConfig
		} else if actualInstance.ArgoCD != nil && actualInstance.ArgoCD.Spec.InstanceSpec.KubeVisionConfig != nil {
			// The server stamps CveScanConfig.RescanInterval when
			// scanEnabled=true. If the CR set CveScanConfig but omitted
			// RescanInterval, inherit the observed scalar so comparison
			// does not flap on the server default.
			mkv := managedInstance.ArgoCD.Spec.InstanceSpec.KubeVisionConfig
			akv := actualInstance.ArgoCD.Spec.InstanceSpec.KubeVisionConfig
			if mkv.CveScanConfig != nil && akv.CveScanConfig != nil {
				if mkv.CveScanConfig.RescanInterval == "" {
					mkv.CveScanConfig.RescanInterval = akv.CveScanConfig.RescanInterval
				}
			} else if mkv.CveScanConfig == nil {
				mkv.CveScanConfig = akv.CveScanConfig
			}
		}

		// The platform may default AkuityIntelligenceExtension.
		if managedInstance.ArgoCD.Spec.InstanceSpec.AkuityIntelligenceExtension == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.AkuityIntelligenceExtension = actualInstance.ArgoCD.Spec.InstanceSpec.AkuityIntelligenceExtension
		}

		// The server always populates AppReconciliationsRateLimiting with a
		// defaulted struct (BucketRateLimiting{Enabled:false, BucketSize:500,
		// BucketQps:50} + ItemRateLimiting{Enabled:true, FailureCooldown:10000,
		// BaseDelay:1, MaxDelay:1000, BackoffFactor:"1.5"}). If the CR omits
		// it entirely, inherit the server's defaults to avoid a per-poll
		// drift-flap. If the CR populates it partially, inherit the
		// missing sibling sub-struct for the same reason.
		if managedInstance.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting = actualInstance.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting
		} else if actualInstance.ArgoCD != nil && actualInstance.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting != nil {
			arl := managedInstance.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting
			aarl := actualInstance.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting
			if arl.ItemRateLimiting == nil {
				arl.ItemRateLimiting = aarl.ItemRateLimiting
			} else {
				normalizeItemRateLimiting(arl.ItemRateLimiting, aarl.ItemRateLimiting)
			}
			if arl.BucketRateLimiting == nil {
				arl.BucketRateLimiting = aarl.BucketRateLimiting
			} else {
				normalizeBucketRateLimiting(arl.BucketRateLimiting, aarl.BucketRateLimiting)
			}
		}

		// IpAllowList is owned by a separate InstanceIpAllowList MR. When
		// the Instance CR omits the field, inherit the server's current
		// list so an IpAllowList MR writing to the same Instance doesn't
		// cause a per-poll Instance Apply flap.
		if managedInstance.ArgoCD.Spec.InstanceSpec.IpAllowList == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.IpAllowList = actualInstance.ArgoCD.Spec.InstanceSpec.IpAllowList
		}

		// ClusterCustomizationDefaults.Kustomization round-trips through
		// the platform as a verbatim string. The server reliably echoes
		// back a trailing "\n" on values that did not have one, even
		// for non-YAML input that bypasses our empty-equivalent check.
		// Without this normalization, "value" and "value\n" would fire
		// ApplyInstance every poll while server-side equality
		// short-circuits the actual write.
		if managedInstance.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults != nil &&
			actualInstance.ArgoCD != nil &&
			actualInstance.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults != nil {
			mk := managedInstance.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults.Kustomization
			ak := actualInstance.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults.Kustomization
			if strings.TrimRight(mk, "\n") == strings.TrimRight(ak, "\n") && mk != ak {
				managedInstance.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults.Kustomization = ak
			}
		}
	}

	// Canonicalize ConfigMap values that the platform stores in a
	// stable order that may differ from user input.
	for k, v := range managedInstance.ArgoCDConfigMap {
		if strings.Contains(k, "accounts.") {
			value := ""
			if strings.Contains(v, "apiKey") {
				value = "apiKey"
			}
			if strings.Contains(v, "login") {
				if value != "" {
					value += ",login"
				} else {
					value = "login"
				}
			}
			if value != "" {
				managedInstance.ArgoCDConfigMap[k] = value
			}
		}

		if k == "resource.customizations" {
			customizations := make(map[string]*ResourceCustomization)
			if err := yaml.Unmarshal([]byte(v), &customizations); err != nil {
				return
			}
			actualCustomizations := make(map[string]*ResourceCustomization)
			if err := yaml.Unmarshal([]byte(actualInstance.ArgoCDConfigMap[k]), &actualCustomizations); err != nil {
				return
			}
			if !cmp.Equal(customizations, actualCustomizations, utilcmp.EquateEmpty()...) {
				return
			}
			managedInstance.ArgoCDConfigMap[k] = actualInstance.ArgoCDConfigMap[k]
		}
	}
	for _, k := range ignoredArgocdCMKeys {
		delete(managedInstance.ArgoCDConfigMap, k)
		delete(actualInstance.ArgoCDConfigMap, k)
	}
}

func normalizeBucketRateLimiting(desired, observed *crossplanetypes.BucketRateLimiting) {
	if desired == nil || observed == nil {
		return
	}
	if desired.Enabled == nil {
		desired.Enabled = observed.Enabled
	}
	// The platform ignores bucketSize/bucketQps while bucket limiting is
	// disabled and returns its default floor. When enabled=true but a scalar
	// was omitted, presence-mode drift already treats that omission as no
	// opinion, so only normalize the disabled shape here.
	if !ptr.Deref(desired.Enabled, false) {
		desired.BucketSize = observed.BucketSize
		desired.BucketQps = observed.BucketQps
	}
}

// normalizeItemRateLimiting mirrors the bucket pattern for the item
// rate-limiting sub-tree. The platform's read path only echoes user-pinned
// scalars when the user also set Enabled=true; when Enabled is omitted or
// false the gateway returns its default scalars (FailureCooldown=10000ms,
// BaseDelay=1ms, MaxDelay=1000ms, BackoffFactor=1.5) regardless of what was
// persisted on Update. Without this absorb a CR pinning Enabled=false with
// non-default scalars drift-flaps every poll because desired=user values vs
// observed=platform defaults.
func normalizeItemRateLimiting(desired, observed *crossplanetypes.ItemRateLimiting) {
	if desired == nil || observed == nil {
		return
	}
	if desired.Enabled == nil {
		desired.Enabled = observed.Enabled
	}
	if !ptr.Deref(desired.Enabled, false) {
		desired.FailureCooldown = observed.FailureCooldown
		desired.BaseDelay = observed.BaseDelay
		desired.MaxDelay = observed.MaxDelay
		desired.BackoffFactorString = observed.BackoffFactorString
	}
}

// ignoredArgocdCMKeys are platform-owned argocd-cm keys that should
// not participate in drift.
var ignoredArgocdCMKeys = []string{
	"cluster.inClusterEnabled",
	"resource.respectRBAC",
	"application.resourceTrackingMethod",
	"url",
}

// driftSpec is the Instance drift-detection recipe. Normalize lifts
// server-defaulted fields onto desired, reorders known-stable
// ConfigMap payloads, and strips keys the server owns. EquateEmpty is
// contributed by the shared DriftSpec baseline.
//
// SecretRef fields (ArgoCD / ArgoCDNotifications / ArgoCDImageUpdater /
// ApplicationSet + the two repo-cred lists) are spec-only. The Akuity
// Export endpoint returns the Secret data masked/nil, so comparing a
// populated desired ref against an observed nil always flags drift and
// an Apply fires every poll. These refs are write-only; the reconcile
// path rotates via status.atProvider.secretHash. Ignore them in the
// struct comparison; drift detection for the referenced secret content
// lives on the hash compare.
//
// Resources is ignored here too: declarative Argo CD children are
// additive, the gateway returns them in separate Export slices
// (Applications / ApplicationSets / AppProjects), and the struct
// compare would flag desired=[...] vs observed=nil forever. The
// argocdResourcesUpToDate side-check on the Export response replaces
// the struct-level comparison for these.
func driftSpec() base.DriftSpec[v1alpha1.InstanceParameters] {
	return base.DriftSpec[v1alpha1.InstanceParameters]{
		Ignore: []cmp.Option{
			cmpopts.IgnoreFields(v1alpha1.InstanceParameters{},
				"ArgoCDSecretRef",
				"ArgoCDNotificationsSecretRef",
				"ArgoCDImageUpdaterSecretRef",
				"ApplicationSetSecretRef",
				"RepoCredentialSecretRefs",
				"RepoTemplateCredentialSecretRefs",
				"Resources",
			),
		},
		Normalize: normalizeInstanceParameters,
	}
}
