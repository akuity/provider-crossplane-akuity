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
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	mock_akuity_client "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/kube"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
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

// newExt constructs an *external with the supplied mock akuity client
// and an optional kube client. Mirrors kargoagent_test.go's newExt.
func newExt(t *testing.T, kube *fake.ClientBuilder) (*external, *mock_akuity_client.MockClient) {
	t.Helper()
	mc := mock_akuity_client.NewMockClient(gomock.NewController(t))
	if kube == nil {
		kube = fake.NewClientBuilder()
	}
	return &external{ExternalClient: base.ExternalClient{
		Client: mc,
		Kube:   kube.Build(),
		Logger: logging.NewNopLogger(),
	}}, mc
}

func TestCreate_NoKubeConfig(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = false
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = v1alpha1.SecretRef{}

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).
		Return(nil).Times(1)

	resp, err := e.Create(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_ApplyClusterErr(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = false
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = v1alpha1.SecretRef{}

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).
		Return(errors.New("fake")).Times(1)

	resp, err := e.Create(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_WithKubeConfig_GetClusterManifestsErr(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = true
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = v1alpha1.SecretRef{}

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).
		Return(nil).Times(1)

	mc.EXPECT().GetClusterManifests(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return("", errors.New("fake")).Times(1)

	resp, err := e.Create(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_WithKubeConfig_ApplyClusterManifestsErr(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = true

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).
		Return(nil).Times(1)

	mc.EXPECT().GetClusterManifests(ctx, managedCluster.Spec.ForProvider.InstanceID, managedCluster.Spec.ForProvider.Name).
		Return("", nil).Times(1)

	resp, err := e.Create(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_GetClusterKubeClientRestConfig(t *testing.T) {
	e, _ := newExt(t, fake.NewClientBuilder().WithRuntimeObjects(kubeconfigSecret))

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = false
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = v1alpha1.SecretRef{
		Name:      "kubeconfig",
		Namespace: "default",
	}

	_, err := kube.RestConfig(ctx, e.Kube, targetKubeConfig(managedCluster))
	require.NoError(t, err)
}

func TestUpdate_ApplyClusterErr(t *testing.T) {
	e, mc := newExt(t, nil)

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).
		Return(errors.New("fake")).Times(1)

	resp, err := e.Update(ctx, &fixtures.CrossplaneManagedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestUpdate(t *testing.T) {
	e, mc := newExt(t, nil)

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).
		Return(nil).Times(1)

	resp, err := e.Update(ctx, &fixtures.CrossplaneManagedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestDelete_NoExternalName(t *testing.T) {
	e, _ := newExt(t, nil)

	resp, err := e.Delete(ctx, &v1alpha1.Cluster{})
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalDelete{}, resp)
}

func TestDelete_RemoveAgentResourcesOnDestroy_GetClusterManifestsErr(t *testing.T) {
	e, mc := newExt(t, nil)

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

	mc.EXPECT().GetClusterManifests(ctx, managedCluster.Spec.ForProvider.InstanceID, managedCluster.Spec.ForProvider.Name).
		Return("", errors.New("fake")).Times(1)

	resp, err := e.Delete(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalDelete{}, resp)
}

func TestDelete_RemoveAgentResourcesOnDestroy_ApplyClusterManifestsErr(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.RemoveAgentResourcesOnDestroy = true
	managedCluster.Spec.ForProvider.EnableInClusterKubeConfig = true

	mc.EXPECT().GetClusterManifests(ctx, managedCluster.Spec.ForProvider.InstanceID, managedCluster.Spec.ForProvider.Name).
		Return("", nil).Times(1)

	resp, err := e.Delete(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalDelete{}, resp)
}

func TestDelete_DeleteClusterErr(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.RemoveAgentResourcesOnDestroy = false

	mc.EXPECT().DeleteCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(errors.New("fake")).Times(1)

	resp, err := e.Delete(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalDelete{}, resp)
}

func TestObserve_EmptyExternalName(t *testing.T) {
	e, _ := newExt(t, nil)

	resp, err := e.Observe(ctx, &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			ForProvider: v1alpha1.ClusterParameters{
				InstanceID: "test",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_InstanceRefNotFoundErr(t *testing.T) {
	e, _ := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.InstanceID = ""
	managedCluster.Spec.ForProvider.InstanceRef = &v1alpha1.LocalReference{
		Name: fixtures.InstanceName,
	}

	resp, err := e.Observe(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
}

func TestObserve_InstanceRefGetInstanceErr(t *testing.T) {
	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Name: fixtures.InstanceName,
	}

	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s) //nolint:errcheck
	kube := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(&managedInstance)
	e, mc := newExt(t, kube)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.InstanceID = ""
	managedCluster.Spec.ForProvider.InstanceRef = &v1alpha1.LocalReference{
		Name: fixtures.InstanceName,
	}

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(nil, errors.New("fake")).Times(1)

	resp, err := e.Observe(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
}

func TestObserve_ClusterNotFoundErr(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}

	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(nil, reason.AsNotFound(errors.New("not found"))).Times(1)

	resp, err := e.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_GetClusterErr(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}

	// The akuity client translates both NotFound and PermissionDenied from
	// the API into reason.NotFound (see internal/reason/doc.go). Observe
	// must treat that as an absent external resource.
	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(nil, reason.AsNotFound(errors.New("cluster not found"))).Times(1)

	resp, err := e.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_GetClusterGenericErrPropagates(t *testing.T) {
	ctx := context.Background()
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}

	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(nil, errors.New("boom")).Times(1)

	resp, err := e.Observe(ctx, &managedCluster)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
	require.NotEmpty(t, managedCluster.Status.Conditions)
	assert.Equal(t, xpv1.ReasonReconcileError, managedCluster.Status.Conditions[0].Reason)
}

func TestObserve_HealthStatusNotHealthy(t *testing.T) {
	e, mc := newExt(t, nil)

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

	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(fixtures.ArgocdCluster, nil).Times(1)
	mc.EXPECT().ExportInstanceByID(ctx, fixtures.InstanceID).
		Return(&argocdv1.ExportInstanceResponse{Clusters: []*structpb.Struct{fixtures.ExportedCluster}}, nil).Times(1)

	_, err := e.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, xpv1.Unavailable().Reason, managedCluster.Status.Conditions[0].Reason)
}

func TestObserve_HealthStatusHealthy(t *testing.T) {
	e, mc := newExt(t, nil)

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

	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(fixtures.ArgocdCluster, nil).Times(1)
	mc.EXPECT().ExportInstanceByID(ctx, fixtures.InstanceID).
		Return(&argocdv1.ExportInstanceResponse{Clusters: []*structpb.Struct{fixtures.ExportedCluster}}, nil).Times(1)

	_, err := e.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, xpv1.Available().Reason, managedCluster.Status.Conditions[0].Reason)
}

func TestObserve_ClusterUpToDate(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}

	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(fixtures.ArgocdCluster, nil).Times(1)
	mc.EXPECT().ExportInstanceByID(ctx, fixtures.InstanceID).
		Return(&argocdv1.ExportInstanceResponse{Clusters: []*structpb.Struct{fixtures.ExportedCluster}}, nil).Times(1)

	resp, err := e.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, resp)
}

func TestObserve_ClusterNotUpToDate(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	managedCluster.Spec.ForProvider.ClusterSpec.Description = "new description"

	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(fixtures.ArgocdCluster, nil).Times(1)
	mc.EXPECT().ExportInstanceByID(ctx, fixtures.InstanceID).
		Return(&argocdv1.ExportInstanceResponse{Clusters: []*structpb.Struct{fixtures.ExportedCluster}}, nil).Times(1)

	resp, err := e.Observe(ctx, &managedCluster)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, resp)
}
