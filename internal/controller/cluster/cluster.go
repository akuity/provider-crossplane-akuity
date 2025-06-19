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

package cluster

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/kube"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/config"
	"github.com/akuityio/provider-crossplane-akuity/internal/features"
	"github.com/akuityio/provider-crossplane-akuity/internal/types"
	utilcmp "github.com/akuityio/provider-crossplane-akuity/internal/utils/cmp"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/pointer"
)

const (
	errNotCluster       = "managed resource is not a Cluster custom resource"
	errTransformCluster = "cannot transform cluster to Akuity API model"
)

// Setup adds a controller that reconciles Cluster managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ClusterGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	logger := o.Logger.WithValues("controller", name)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ClusterGroupVersionKind),
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
		For(&v1alpha1.Cluster{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube   client.Client
	usage  resource.Tracker
	logger logging.Logger
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return nil, errors.New(errNotCluster)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, "cannot track ProviderConfig usage")
	}

	client, err := config.GetAkuityClientFromProviderConfig(ctx, c.kube, cr.GetProviderConfigReference().Name)
	if err != nil {
		return nil, err
	}

	return NewExternal(client, c.kube, c.logger), nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type External struct {
	client akuity.Client
	kube   client.Client
	logger logging.Logger
}

func NewExternal(client akuity.Client, kube client.Client, logger logging.Logger) *External {
	return &External{
		client: client,
		kube:   kube,
		logger: logger,
	}
}

func (c *External) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	managedCluster, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotCluster)
	}

	instanceID, err := c.getInstanceID(ctx, managedCluster.Spec.ForProvider.InstanceID, managedCluster.Spec.ForProvider.InstanceRef)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	managedCluster.Spec.ForProvider.InstanceID = instanceID

	if meta.GetExternalName(managedCluster) == "" {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	akuityCluster, err := c.client.GetCluster(ctx, instanceID, meta.GetExternalName(managedCluster))
	if err != nil {
		c.logger.Debug("As we are not able to differentiate PermissionDenied/NotFound when calling GetCluster, we simply treat as not found here", "error", err)
		return managed.ExternalObservation{ResourceExists: false}, nil

		// As mentioned above, we simply treat PermissionDenied/NotFound as not found error here, otherwise, the provider
		// is not able to create cluster.

		// if reason.IsNotFound(err) {
		// 	return managed.ExternalObservation{ResourceExists: false}, nil
		// }
		//
		// managedCluster.SetConditions(xpv1.ReconcileError(err))
		// return managed.ExternalObservation{}, err
	}

	actualCluster, err := types.AkuityAPIToCrossplaneCluster(instanceID, managedCluster.Spec.ForProvider, akuityCluster)
	if err != nil {
		newErr := fmt.Errorf("could not transform cluster from Akuity API: %w", err)
		managedCluster.SetConditions(xpv1.ReconcileError(newErr))
		return managed.ExternalObservation{}, newErr
	}

	lateInitializeCluster(&managedCluster.Spec.ForProvider, actualCluster)

	clusterObservation, err := types.AkuityAPIToCrossplaneClusterObservation(akuityCluster)
	if err != nil {
		newErr := fmt.Errorf("could not transform cluster observation: %w", err)
		managedCluster.SetConditions(xpv1.ReconcileError(newErr))
		return managed.ExternalObservation{}, newErr
	}

	managedCluster.Status.AtProvider = clusterObservation

	if clusterObservation.HealthStatus.Code != 1 {
		managedCluster.SetConditions(xpv1.Unavailable())
	} else {
		managedCluster.SetConditions(xpv1.Available())
	}

	isUpToDate := checkClusterUpToDate(managedCluster.Spec.ForProvider, actualCluster)

	if !isUpToDate {
		c.logger.Debug("Comparing managed cluster to external instance", "diff", cmp.Diff(managedCluster.Spec.ForProvider, actualCluster))
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate,
	}, nil
}

func (c *External) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	managedCluster, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotCluster)
	}

	akuityAPICluster, err := types.CrossplaneToAkuityAPICluster(managedCluster.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.New(errTransformCluster)
	}

	err = c.client.ApplyCluster(ctx, managedCluster.Spec.ForProvider.InstanceID, akuityAPICluster)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("could not create cluster: %w", err)
	}

	if managedCluster.Spec.ForProvider.EnableInClusterKubeConfig || managedCluster.Spec.ForProvider.KubeConfigSecretRef.Name != "" {
		c.logger.Debug("Retrieving cluster manifests....")
		clusterManifests, err := c.client.GetClusterManifests(ctx, managedCluster.Spec.ForProvider.InstanceID, managedCluster.Spec.ForProvider.Name)
		if err != nil {
			return managed.ExternalCreation{}, fmt.Errorf("could not get cluster manifests to apply: %w", err)
		}

		c.logger.Debug("Applying cluster manifests",
			"clusterName", managedCluster.Name,
			"instanceID", managedCluster.Spec.ForProvider.InstanceID,
		)
		c.logger.Debug(clusterManifests)
		err = c.applyClusterManifests(ctx, *managedCluster, clusterManifests, false)
		if err != nil {
			return managed.ExternalCreation{}, fmt.Errorf("could not apply cluster manifests: %w", err)
		}
	}
	meta.SetExternalName(managedCluster, akuityAPICluster.Name)

	return managed.ExternalCreation{}, err
}

func (c *External) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	managedCluster, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotCluster)
	}

	akuityAPICluster, err := types.CrossplaneToAkuityAPICluster(managedCluster.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, errors.New(errTransformCluster)
	}

	err = c.client.ApplyCluster(ctx, managedCluster.Spec.ForProvider.InstanceID, akuityAPICluster)

	return managed.ExternalUpdate{}, err
}

func (c *External) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	managedCluster, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotCluster)
	}

	externalName := meta.GetExternalName(managedCluster)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	if managedCluster.Spec.ForProvider.RemoveAgentResourcesOnDestroy &&
		(managedCluster.Spec.ForProvider.EnableInClusterKubeConfig || managedCluster.Spec.ForProvider.KubeConfigSecretRef.Name != "") {
		clusterManifests, err := c.client.GetClusterManifests(ctx, managedCluster.Spec.ForProvider.InstanceID, managedCluster.Spec.ForProvider.Name)
		if err != nil {
			return managed.ExternalDelete{}, fmt.Errorf("could not get cluster manifests to delete: %w", err)
		}

		err = c.applyClusterManifests(ctx, *managedCluster, clusterManifests, true)
		if err != nil {
			return managed.ExternalDelete{}, fmt.Errorf("could not delete cluster manifests: %w", err)
		}
	}

	err := c.client.DeleteCluster(ctx, managedCluster.Spec.ForProvider.InstanceID, externalName)
	if err != nil {
		return managed.ExternalDelete{}, fmt.Errorf("could not delete cluster: %w", err)
	}

	return managed.ExternalDelete{}, nil
}

func (c *External) Disconnect(ctx context.Context) error {
	return nil
}

func lateInitializeCluster(in *v1alpha1.ClusterParameters, actual v1alpha1.ClusterParameters) {
	in.Namespace = pointer.LateInitialize(in.Namespace, actual.Namespace)
	in.ClusterSpec.Data.AutoUpgradeDisabled = pointer.LateInitialize(in.ClusterSpec.Data.AutoUpgradeDisabled, actual.ClusterSpec.Data.AutoUpgradeDisabled)
	in.ClusterSpec.Data.AppReplication = pointer.LateInitialize(in.ClusterSpec.Data.AppReplication, actual.ClusterSpec.Data.AppReplication)
	in.ClusterSpec.Data.TargetVersion = pointer.LateInitialize(in.ClusterSpec.Data.TargetVersion, actual.ClusterSpec.Data.TargetVersion)
	in.ClusterSpec.Data.RedisTunneling = pointer.LateInitialize(in.ClusterSpec.Data.RedisTunneling, actual.ClusterSpec.Data.RedisTunneling)
	in.ClusterSpec.Data.Size = pointer.LateInitialize(in.ClusterSpec.Data.Size, actual.ClusterSpec.Data.Size)
	in.ClusterSpec.Data.Kustomization = pointer.LateInitialize(in.ClusterSpec.Data.Kustomization, actual.ClusterSpec.Data.Kustomization)
	in.ClusterSpec.Data.MultiClusterK8SDashboardEnabled = pointer.LateInitialize(in.ClusterSpec.Data.MultiClusterK8SDashboardEnabled, actual.ClusterSpec.Data.MultiClusterK8SDashboardEnabled)
	in.ClusterSpec.Data.AutoscalerConfig = pointer.LateInitialize(in.ClusterSpec.Data.AutoscalerConfig, actual.ClusterSpec.Data.AutoscalerConfig)
}

func (c *External) getInstanceID(ctx context.Context, instanceID string, instanceRef v1alpha1.NameRef) (string, error) {
	if instanceID != "" {
		return instanceID, nil
	}

	if instanceRef.Name == "" {
		return "", fmt.Errorf("one of InstanceID or InstanceRef must be provided")
	}

	instance := &v1alpha1.Instance{}
	if err := c.kube.Get(ctx, k8stypes.NamespacedName{Name: instanceRef.Name}, instance); err != nil {
		return "", fmt.Errorf("could not look up instance with instanceRef %s: %w", instanceRef.Name, err)
	}

	akuityInstance, err := c.client.GetInstance(ctx, instance.Spec.ForProvider.Name)
	if err != nil {
		return "", fmt.Errorf("could not look up instance with instanceRef %s: %w", instanceRef.Name, err)
	}

	return akuityInstance.GetId(), nil
}

func (c *External) GetClusterKubeClientRestConfig(ctx context.Context, managedCluster v1alpha1.Cluster) (*rest.Config, error) {
	var restConfig *rest.Config
	var err error
	if managedCluster.Spec.ForProvider.EnableInClusterKubeConfig {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return restConfig, fmt.Errorf("could not build in cluster kube config: %w", err)
		}
	} else {
		secretName := managedCluster.Spec.ForProvider.KubeConfigSecretRef.Name
		secretNamespace := managedCluster.Spec.ForProvider.KubeConfigSecretRef.Namespace
		secret := &corev1.Secret{}
		if err := c.kube.Get(ctx, k8stypes.NamespacedName{Name: secretName, Namespace: managedCluster.Spec.ForProvider.KubeConfigSecretRef.Namespace}, secret); err != nil {
			return restConfig, fmt.Errorf("could not get secret %s in namespace %s containing cluster kubeconfig: %w", secretName, secretNamespace, err)
		}

		kubeConfig, ok := secret.Data["kubeconfig"]
		if !ok {
			return restConfig, fmt.Errorf("could not get cluster kubeconfig from secret %s in namespace %s", secretName, secretNamespace)
		}

		restConfig, err = clientcmd.RESTConfigFromKubeConfig(kubeConfig)
		if err != nil {
			return restConfig, fmt.Errorf("could not get rest config from kubeconfig in secret %s in namespace %s: %w", secretName, secretNamespace, err)
		}
	}

	return restConfig, nil
}

func (c *External) applyClusterManifests(ctx context.Context, managedCluster v1alpha1.Cluster, clusterManifests string, delete bool) error {
	restConfig, err := c.GetClusterKubeClientRestConfig(ctx, managedCluster)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error creating dynamic client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error creating typed client: %w", err)
	}

	applyClient, err := kube.NewApplyClient(dynamicClient, clientset, c.logger)
	if err != nil {
		return fmt.Errorf("error creating apply client: %w", err)
	}

	return applyClient.ApplyManifests(ctx, clusterManifests, delete)
}

func normalizeClusterParameters(managedCluster, actualCluster *v1alpha1.ClusterParameters) {
	if managedCluster == nil || actualCluster == nil {
		return
	}

	// MultiClusterK8SDashboardEnabled may be enabled by default and not specified in the CR.
	if managedCluster.ClusterSpec.Data.MultiClusterK8SDashboardEnabled == nil {
		managedCluster.ClusterSpec.Data.MultiClusterK8SDashboardEnabled =
			actualCluster.ClusterSpec.Data.MultiClusterK8SDashboardEnabled
	}
}

func checkClusterUpToDate(managedCluster, actualCluster v1alpha1.ClusterParameters) bool {
	normalizeClusterParameters(&managedCluster, &actualCluster)
	return cmp.Equal(managedCluster, actualCluster, utilcmp.EquateEmpty()...)
}
