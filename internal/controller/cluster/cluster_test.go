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

package cluster_test

import (
	"context"
	"testing"

	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	mock_akuity_client "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/cluster"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
)

var (
	ctx        = context.TODO()
	kubeconfig = `
apiVersion: v1
kind: Config
clusters:
- name: minikube
  cluster:
    certificate-authority-data: fake
    server: https://192.168.39.217:8443
users:
- name: minikube
  user:
    client-certificate-data: fake
    client-key-data: fake
contexts:
- name: minikube
  context:
    cluster: minikube
    namespace: default
    user: minikube
current-context: minikube`

	kubeconfigSecret = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"kubeconfig": []byte(kubeconfig),
		},
	}
)

func TestCreate_NotClusterErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	resp, err := client.Create(ctx, &v1alpha1.Instance{})
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_NoKubeConfig(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = false
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = v1alpha1.SecretRef{}

	mockAkuityClient.EXPECT().ApplyCluster(ctx, fixtures.InstanceID, fixtures.AkuityCluster).
		Return(nil).Times(1)

	resp, err := client.Create(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_ApplyClusterErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = false
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = v1alpha1.SecretRef{}

	mockAkuityClient.EXPECT().ApplyCluster(ctx, fixtures.InstanceID, fixtures.AkuityCluster).
		Return(errors.New("fake")).Times(1)

	resp, err := client.Create(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_WithKubeConfig_GetClusterManifestsErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = true
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = v1alpha1.SecretRef{}

	mockAkuityClient.EXPECT().ApplyCluster(ctx, fixtures.InstanceID, fixtures.AkuityCluster).
		Return(nil).Times(1)

	mockAkuityClient.EXPECT().GetClusterManifests(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return("", errors.New("fake")).Times(1)

	resp, err := client.Create(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_WithKubeConfig_ApplyClusterManifestsErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = true

	mockAkuityClient.EXPECT().ApplyCluster(ctx, fixtures.InstanceID, fixtures.AkuityCluster).
		Return(nil).Times(1)

	mockAkuityClient.EXPECT().GetClusterManifests(ctx, managedCluster.Spec.ForProvider.InstanceID, managedCluster.Spec.ForProvider.Name).
		Return("", nil).Times(1)

	resp, err := client.Create(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_GetClusterKubeClientRestConfig(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().WithRuntimeObjects(kubeconfigSecret).Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = false
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = v1alpha1.SecretRef{
		Name:      "kubeconfig",
		Namespace: "default",
	}

	_, err := client.GetClusterKubeClientRestConfig(ctx, managedCluster)
	require.NoError(t, err)
}

func TestUpdate_NotClusterErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	resp, err := client.Update(ctx, &v1alpha1.Instance{})
	require.Error(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestUpdate_ApplyClusterErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	mockAkuityClient.EXPECT().ApplyCluster(ctx, fixtures.InstanceID, fixtures.AkuityCluster).
		Return(errors.New("fake")).Times(1)

	resp, err := client.Update(ctx, &fixtures.CrossplaneManagedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestUpdate(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	mockAkuityClient.EXPECT().ApplyCluster(ctx, fixtures.InstanceID, fixtures.AkuityCluster).
		Return(nil).Times(1)

	resp, err := client.Update(ctx, &fixtures.CrossplaneManagedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestDelete_NotClusterErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	err := client.Delete(ctx, &v1alpha1.Instance{})
	require.Error(t, err)
}

func TestDelete_NoExternalName(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := &v1alpha1.Cluster{}

	err := client.Delete(ctx, managedCluster)
	require.NoError(t, err)
}

func TestDelete_RemoveAgentResourcesOnDestroy_GetClusterManifestsErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.RemoveAgentResourcesOnDestroy = true
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = false
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = v1alpha1.SecretRef{
		Name:      "kubeconfig",
		Namespace: "default",
	}

	mockAkuityClient.EXPECT().GetClusterManifests(ctx, managedCluster.Spec.ForProvider.InstanceID, managedCluster.Spec.ForProvider.Name).
		Return("", errors.New("fake")).Times(1)

	err := client.Delete(ctx, &managedCluster)
	require.Error(t, err)
}

func TestDelete_RemoveAgentResourcesOnDestroy_ApplyClusterManifestsErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.RemoveAgentResourcesOnDestroy = true
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = true

	mockAkuityClient.EXPECT().GetClusterManifests(ctx, managedCluster.Spec.ForProvider.InstanceID, managedCluster.Spec.ForProvider.Name).
		Return("", nil).Times(1)

	err := client.Delete(ctx, &managedCluster)
	require.Error(t, err)
}

func TestDelete_DeleteClusterErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.RemoveAgentResourcesOnDestroy = false

	mockAkuityClient.EXPECT().DeleteCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(errors.New("fake")).Times(1)

	err := client.Delete(ctx, &managedCluster)
	require.Error(t, err)
}

func TestObserve_NotClusterErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	resp, err := client.Observe(ctx, &v1alpha1.Instance{})
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
}

func TestObserve_EmptyExternalName(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	resp, err := client.Observe(ctx, &v1alpha1.Cluster{})
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_InstanceRefNotFoundErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.InstanceID = ""
	managedCluster.Spec.ForProvider.InstanceRef = v1alpha1.NameRef{
		Name: fixtures.InstanceName,
	}

	resp, err := client.Observe(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
}

func TestObserve_InstanceRefGetInstanceErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Name: fixtures.InstanceName,
	}

	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s)
	kube := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(&managedInstance).Build()
	client := cluster.NewExternal(mockAkuityClient, kube, logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.InstanceID = ""
	managedCluster.Spec.ForProvider.InstanceRef = v1alpha1.NameRef{
		Name: fixtures.InstanceName,
	}

	mockAkuityClient.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(nil, errors.New("fake")).Times(1)

	resp, err := client.Observe(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
}

func TestObserve_ClusterNotFoundErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}

	mockAkuityClient.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(nil, reason.AsNotFound(errors.New("not found"))).Times(1)

	resp, err := client.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_GetClusterErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}

	mockAkuityClient.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(nil, errors.New("fake")).Times(1)

	resp, err := client.Observe(ctx, &managedCluster)
	// require.Error(t, err)
	// assert.Equal(t, managed.ExternalObservation{}, resp)
	// assert.Equal(t, xpv1.ReasonReconcileError, managedCluster.Status.Conditions[0].Reason)

	// we use not found for permission denied error
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_HealthStatusNotHealthy(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	argocdCluster := fixtures.ArgocdCluster
	argocdCluster.HealthStatus = &health.Status{
		Code:    health.StatusCode_STATUS_CODE_DEGRADED,
		Message: "Cluster is degraded",
	}

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}

	mockAkuityClient.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(fixtures.ArgocdCluster, nil).Times(1)

	_, err := client.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, xpv1.Unavailable().Reason, managedCluster.Status.Conditions[0].Reason)
}

func TestObserve_HealthStatusHealthy(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	argocdCluster := fixtures.ArgocdCluster
	argocdCluster.HealthStatus = &health.Status{
		Code:    health.StatusCode_STATUS_CODE_HEALTHY,
		Message: "Cluster is healthy",
	}

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}

	mockAkuityClient.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(fixtures.ArgocdCluster, nil).Times(1)

	_, err := client.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, xpv1.Available().Reason, managedCluster.Status.Conditions[0].Reason)
}

func TestObserve_ClusterUpToDate(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}

	mockAkuityClient.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(fixtures.ArgocdCluster, nil).Times(1)

	resp, err := client.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, resp)
}

func TestObserve_ClusterNotUpToDate(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := cluster.NewExternal(mockAkuityClient, fake.NewClientBuilder().Build(), logging.NewNopLogger())

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.ClusterSpec.Description = "new description"

	mockAkuityClient.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(fixtures.ArgocdCluster, nil).Times(1)

	resp, err := client.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, resp)
}
