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

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	utilcmp "github.com/akuityio/provider-crossplane-akuity/internal/utils/cmp"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/config"
	"github.com/akuityio/provider-crossplane-akuity/internal/features"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	"github.com/akuityio/provider-crossplane-akuity/internal/types"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/pointer"
)

const (
	errNotInstance       = "managed resource is not a Instance custom resource"
	errTransformInstance = "cannot transform Crossplane instance to Akuity API instance"
)

// Setup adds a controller that reconciles Instance managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.InstanceGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	logger := o.Logger.WithValues("controller", name)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.InstanceGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:   mgr.GetClient(),
			usage:  resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			logger: logger,
		}),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithInitializers(),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Instance{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube   client.Client
	usage  resource.Tracker
	logger logging.Logger
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Instance)
	if !ok {
		return nil, errors.New(errNotInstance)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, "cannot track ProviderConfig usage")
	}

	client, err := config.GetAkuityClientFromProviderConfig(ctx, c.kube, cr.GetProviderConfigReference().Name)
	if err != nil {
		return nil, err
	}

	return NewExternal(client, c.logger), nil
}

type External struct {
	client akuity.Client
	logger logging.Logger
}

func NewExternal(client akuity.Client, logger logging.Logger) *External {
	return &External{
		client: client,
		logger: logger,
	}
}

func (c *External) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	managedInstance, ok := mg.(*v1alpha1.Instance)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotInstance)
	}

	if meta.GetExternalName(managedInstance) == "" {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	akuityInstance, err := c.client.GetInstance(ctx, meta.GetExternalName(managedInstance))
	if err != nil {
		if reason.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}

		managedInstance.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	akuityExportedInstance, err := c.client.ExportInstance(ctx, meta.GetExternalName(managedInstance))
	if err != nil {
		managedInstance.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	actualInstance, err := types.AkuityAPIToCrossplaneInstance(akuityInstance, akuityExportedInstance)
	if err != nil {
		newErr := fmt.Errorf("could not transform instance spec from Akuity API to internal instance spec: %w", err)
		managedInstance.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, newErr
	}

	err = lateInitializeInstance(&managedInstance.Spec.ForProvider, akuityInstance, akuityExportedInstance)
	if err != nil {
		managedInstance.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	instanceObservation, err := types.AkuityAPIToCrossplaneInstanceObservation(akuityInstance, akuityExportedInstance)
	if err != nil {
		newErr := fmt.Errorf("could not transform instance from Akuity API to Crossplane instance observation: %w", err)
		managedInstance.SetConditions(xpv1.ReconcileError(newErr))
		return managed.ExternalObservation{}, newErr
	}

	managedInstance.Status.AtProvider = instanceObservation

	if instanceObservation.HealthStatus.Code != 1 {
		managedInstance.SetConditions(xpv1.Unavailable())
	} else {
		managedInstance.SetConditions(xpv1.Available())
	}

	c.logger.Debug("Comparing managed instance to external instance", "diff", cmp.Diff(managedInstance.Spec.ForProvider, actualInstance.Spec.ForProvider))

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: cmp.Equal(managedInstance.Spec.ForProvider, actualInstance.Spec.ForProvider, utilcmp.EquateEmpty()...),
	}, nil
}

func (c *External) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	managedInstance, ok := mg.(*v1alpha1.Instance)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotInstance)
	}

	request, err := c.client.BuildApplyInstanceRequest(*managedInstance)
	if err != nil {
		return managed.ExternalCreation{}, errors.New(errTransformInstance)
	}

	err = c.client.ApplyInstance(ctx, request)
	meta.SetExternalName(managedInstance, request.GetId())

	return managed.ExternalCreation{}, err
}

func (c *External) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	managedInstance, ok := mg.(*v1alpha1.Instance)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotInstance)
	}

	request, err := c.client.BuildApplyInstanceRequest(*managedInstance)
	if err != nil {
		return managed.ExternalUpdate{}, errors.New(errTransformInstance)
	}

	err = c.client.ApplyInstance(ctx, request)
	return managed.ExternalUpdate{}, err
}

func (c *External) Delete(ctx context.Context, mg resource.Managed) error {
	managedInstance, ok := mg.(*v1alpha1.Instance)
	if !ok {
		return errors.New(errNotInstance)
	}

	externalName := meta.GetExternalName(managedInstance)
	if externalName == "" {
		return nil
	}

	return c.client.DeleteInstance(ctx, externalName)
}

func lateInitializeInstance(in *v1alpha1.InstanceParameters, instance *argocdv1.Instance, exportedInstance *argocdv1.ExportInstanceResponse) error {
	in.ArgoCD.Spec.InstanceSpec.Subdomain = pointer.LateInitialize(in.ArgoCD.Spec.InstanceSpec.Subdomain, instance.GetSpec().GetSubdomain())
	in.ArgoCD.Spec.InstanceSpec.DeclarativeManagementEnabled = pointer.LateInitialize(in.ArgoCD.Spec.InstanceSpec.DeclarativeManagementEnabled, instance.GetSpec().GetDeclarativeManagementEnabled())
	in.ArgoCD.Spec.InstanceSpec.AppsetPolicy = pointer.LateInitialize(in.ArgoCD.Spec.InstanceSpec.AppsetPolicy, types.AkuityAPIToCrossplaneAppsetPolicy(instance.GetSpec().GetAppsetPolicy()))

	if in.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults == nil || in.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults.Kustomization == "" {
		clusterCustomizationDefaults, err := types.AkuityAPIToCrossplaneClusterCustomization(instance.GetSpec().GetClusterCustomizationDefaults())
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
		argocdConfigMap, err := types.AkuityAPIConfigMapToMap(types.ARGOCD_CM_KEY, exportedInstance.GetArgocdConfigmap())
		if err != nil {
			return fmt.Errorf("could not late initialize instance configmap %s: %w", types.ARGOCD_CM_KEY, err)
		}
		in.ArgoCDConfigMap = argocdConfigMap
	}

	if in.ArgoCDSSHKnownHostsConfigMap == nil {
		argocdKnownHostsConfigMap, err := types.AkuityAPIConfigMapToMap(types.ARGOCD_SSH_KNOWN_HOSTS_CM_KEY, exportedInstance.GetArgocdKnownHostsConfigmap())
		if err != nil {
			return fmt.Errorf("could not late initialize instance configmap %s: %w", types.ARGOCD_SSH_KNOWN_HOSTS_CM_KEY, err)
		}
		in.ArgoCDSSHKnownHostsConfigMap = argocdKnownHostsConfigMap
	}

	if in.ArgoCDRBACConfigMap == nil {
		argocdRBACConfigMap, err := types.AkuityAPIConfigMapToMap(types.ARGOCD_RBAC_CM_KEY, exportedInstance.GetArgocdRbacConfigmap())
		if err != nil {
			return fmt.Errorf("could not late initialize instance configmap %s: %w", types.ARGOCD_RBAC_CM_KEY, err)
		}
		in.ArgoCDRBACConfigMap = argocdRBACConfigMap
	}

	if in.ArgoCDTLSCertsConfigMap == nil {
		argocdTLSCertsConfigMap, err := types.AkuityAPIConfigMapToMap(types.ARGOCD_TLS_CERTS_CM_KEY, exportedInstance.GetArgocdTlsCertsConfigmap())
		if err != nil {
			return fmt.Errorf("could not late initialize instance configmap %s: %w", types.ARGOCD_TLS_CERTS_CM_KEY, err)
		}
		in.ArgoCDTLSCertsConfigMap = argocdTLSCertsConfigMap
	}

	if in.ConfigManagementPlugins == nil {
		configManagementPlugins, err := types.AkuityAPIToCrossplaneConfigManagementPlugins(exportedInstance.GetConfigManagementPlugins())
		if err != nil {
			return fmt.Errorf("could not late initialize instance config management plugins: %w", err)
		}
		in.ConfigManagementPlugins = configManagementPlugins
	}

	return nil
}
