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
	"github.com/pkg/errors"
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

	mg.Status.AtProvider = instanceObservation
	base.SetHealthCondition(mg, instanceObservation.HealthStatus.Code == 1)

	// DeepCopy so Normalize's map mutations (ArgoCDConfigMap rewrites,
	// ignored-key deletions) don't leak back into the managed resource.
	spec := driftSpec()
	desired := mg.Spec.ForProvider.DeepCopy()
	observed := actualInstance.Spec.ForProvider.DeepCopy()
	isUpToDate, err := base.EvaluateDrift(ctx, spec, desired, observed, e.Logger, "Instance")
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate,
	}, nil
}

func (e *external) Create(ctx context.Context, mg *v1alpha1.Instance) (managed.ExternalCreation, error) {
	defer base.PropagateObservedGeneration(mg)

	request, err := BuildApplyInstanceRequest(*mg)
	if err != nil {
		return managed.ExternalCreation{}, errors.New(errTransformInstance)
	}

	err = e.Client.ApplyInstance(ctx, request)
	meta.SetExternalName(mg, request.GetId())

	return managed.ExternalCreation{}, err
}

func (e *external) Update(ctx context.Context, mg *v1alpha1.Instance) (managed.ExternalUpdate, error) {
	defer base.PropagateObservedGeneration(mg)

	request, err := BuildApplyInstanceRequest(*mg)
	if err != nil {
		return managed.ExternalUpdate{}, errors.New(errTransformInstance)
	}

	return managed.ExternalUpdate{}, e.Client.ApplyInstance(ctx, request)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha1.Instance) (managed.ExternalDelete, error) {
	defer base.PropagateObservedGeneration(mg)

	externalName := meta.GetExternalName(mg)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	return managed.ExternalDelete{}, e.Client.DeleteInstance(ctx, externalName)
}

func (e *external) Disconnect(_ context.Context) error { return nil }

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

// normalizeInstanceParameters synchronizes default values from the actual instance to the managed instance,
// and normalize fields that have same values as the actual instance. This ensures consistency with API defaults.
func normalizeInstanceParameters(managedInstance, actualInstance *v1alpha1.InstanceParameters) { //nolint:gocyclo
	if managedInstance == nil || actualInstance == nil {
		return
	}
	if managedInstance.ArgoCD != nil {
		// MultiClusterK8SDashboardEnabled may be enabled by default and not specified in the CR.
		if managedInstance.ArgoCD.Spec.InstanceSpec.MultiClusterK8SDashboardEnabled == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.MultiClusterK8SDashboardEnabled = actualInstance.ArgoCD.Spec.InstanceSpec.MultiClusterK8SDashboardEnabled
		}

		// only one of Fqdn and Subdomain should be set, so we sync them if both are set
		if managedInstance.ArgoCD.Spec.InstanceSpec.Fqdn != "" && managedInstance.ArgoCD.Spec.InstanceSpec.Subdomain != "" {
			managedInstance.ArgoCD.Spec.InstanceSpec.Subdomain = actualInstance.ArgoCD.Spec.InstanceSpec.Subdomain
			managedInstance.ArgoCD.Spec.InstanceSpec.Fqdn = actualInstance.ArgoCD.Spec.InstanceSpec.Fqdn
		}

		if managedInstance.ArgoCD.Spec.InstanceSpec.KubeVisionConfig == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.KubeVisionConfig = actualInstance.ArgoCD.Spec.InstanceSpec.KubeVisionConfig
		}

		// If Akuity Intelligence is enabled by default, sync the value
		if managedInstance.ArgoCD.Spec.InstanceSpec.AkuityIntelligenceExtension == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.AkuityIntelligenceExtension = actualInstance.ArgoCD.Spec.InstanceSpec.AkuityIntelligenceExtension
		}

		// The server always populates AppReconciliationsRateLimiting with a
		// defaulted struct (BucketRateLimiting{Enabled:false, BucketSize:500,
		// BucketQps:50} + ItemRateLimiting{Enabled:true, FailureCooldown:10000,
		// BaseDelay:1, MaxDelay:1000, BackoffFactor:"1.5"}). If the CR omits
		// it entirely, inherit the server's defaults to avoid a per-poll
		// drift-flap. Users who populate the struct partially get their
		// explicit values honoured; scalar-field defaulting inside a
		// partially-set struct is Tier 2 territory (§6 #7 scalar lateInit
		// gap).
		if managedInstance.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting = actualInstance.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting
		}

		// IpAllowList is owned by a separate InstanceIpAllowList MR. When
		// the Instance CR omits the field, inherit the server's current
		// list so an IpAllowList MR writing to the same Instance doesn't
		// cause a per-poll Instance Apply flap (1C.D9 coexist case).
		if managedInstance.ArgoCD.Spec.InstanceSpec.IpAllowList == nil {
			managedInstance.ArgoCD.Spec.InstanceSpec.IpAllowList = actualInstance.ArgoCD.Spec.InstanceSpec.IpAllowList
		}
	}

	// some configmap values have stable orders which may not be the same as user input
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
	}
}

// some values are not able to be configured, and we ignore them if they are set
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
// ApplicationSet + the two repo-cred lists) are spec-only — the Akuity
// Export endpoint returns the Secret data masked/nil, so comparing a
// populated desired ref against an observed nil always flags drift and
// an Apply fires every poll. These refs are write-only per §2.11; the
// reconcile path rotates via status.atProvider.secretHash (when the
// secret-resolution plumbing lands). Ignore them in the struct
// comparison; drift detection for the referenced secret content lives
// on the hash compare.
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
			),
		},
		Normalize: normalizeInstanceParameters,
	}
}
