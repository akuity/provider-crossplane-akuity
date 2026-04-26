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
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	mock_akuity_client "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/kube"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	generated "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
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
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = xpv1.SecretReference{}

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
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = xpv1.SecretReference{}

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
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = xpv1.SecretReference{}

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
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = xpv1.SecretReference{
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

// TestUpdate_MaintenanceModeSetCallsSetEndpoint covers the SET case:
// when the user populates data.maintenanceMode (and optionally
// data.maintenanceModeExpiry), Update must Apply the rest of the
// cluster spec AND call SetClusterMaintenanceMode exactly once with
// the user-set values. Without this, ApplyInstance silently drops the
// maintenance fields, the platform never enters maintenance, and the
// drift comparator fires Apply on every poll.
func TestUpdate_MaintenanceModeSetCallsSetEndpoint(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.ClusterSpec.Data.MaintenanceMode = ptr.To(true)
	expiryStr := "2027-01-15T03:30:00Z"
	managedCluster.Spec.ForProvider.ClusterSpec.Data.MaintenanceModeExpiry = ptr.To(expiryStr)

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).Return(nil).Times(1)
	mc.EXPECT().
		SetClusterMaintenanceMode(ctx, fixtures.InstanceID, fixtures.ClusterName, true, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ string, mode bool, expiry *time.Time) error {
			assert.True(t, mode)
			require.NotNil(t, expiry)
			parsed, err := time.Parse(time.RFC3339, expiryStr)
			require.NoError(t, err)
			assert.True(t, expiry.Equal(parsed),
				"expiry sent through SetClusterMaintenanceMode must match the spec value")
			return nil
		}).Times(1)

	_, err := e.Update(ctx, &managedCluster)
	require.NoError(t, err)
}

// TestUpdate_MaintenanceModeFlipCallsSetEndpoint covers the FLIP case:
// toggling the maintenance flag must produce exactly one new
// SetClusterMaintenanceMode call with the new value.
func TestUpdate_MaintenanceModeFlipCallsSetEndpoint(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.ClusterSpec.Data.MaintenanceMode = ptr.To(false)

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).Return(nil).Times(1)
	mc.EXPECT().
		SetClusterMaintenanceMode(ctx, fixtures.InstanceID, fixtures.ClusterName, false, nil).
		Return(nil).Times(1)

	_, err := e.Update(ctx, &managedCluster)
	require.NoError(t, err)
}

// TestUpdate_MaintenanceModeUnsetSkipsSetEndpoint covers the skip:
// when neither MaintenanceMode nor MaintenanceModeExpiry is configured,
// Update must not call the dedicated endpoint. Implicitly clearing a
// value the user didn't ask to control would silently override
// out-of-band UI changes.
func TestUpdate_MaintenanceModeUnsetSkipsSetEndpoint(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.ClusterSpec.Data.MaintenanceMode = nil
	managedCluster.Spec.ForProvider.ClusterSpec.Data.MaintenanceModeExpiry = nil

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).Return(nil).Times(1)
	// gomock.NewController fails on unexpected calls; no expectation
	// here doubles as an assertion that SetClusterMaintenanceMode is
	// never invoked when the field is unset.

	_, err := e.Update(ctx, &managedCluster)
	require.NoError(t, err)
}

// TestUpdate_MaintenanceModeBadExpiryFails locks the parse error in
// place: a non-RFC3339 expiry string must surface as a reconcile error
// rather than silently ignoring the user's input.
func TestUpdate_MaintenanceModeBadExpiryFails(t *testing.T) {
	e, mc := newExt(t, nil)

	managedCluster := fixtures.CrossplaneManagedCluster
	managedCluster.Spec.ForProvider.ClusterSpec.Data.MaintenanceMode = ptr.To(true)
	managedCluster.Spec.ForProvider.ClusterSpec.Data.MaintenanceModeExpiry = ptr.To("not-a-timestamp")

	mc.EXPECT().ApplyInstance(ctx, gomock.Any()).Return(nil).Times(1)
	// SetClusterMaintenanceMode must NOT be called when expiry parsing fails.

	_, err := e.Update(ctx, &managedCluster)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maintenanceModeExpiry")
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
	managedCluster.Spec.ForProvider.KubeConfigSecretRef = xpv1.SecretReference{
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

// observeFixtureWithAutoscaler builds a Cluster MR + matching gateway
// mocks for the AutoscalerConfig drift scenarios in Issue 2. The
// helper centralises the boilerplate (external-name annotation,
// per-test Get/Export plumbing) so each case stays focused on the
// pointer-field semantics under test.
func observeFixtureWithAutoscaler(
	t *testing.T,
	desired *argocdv1.AutoScalerConfig, // applied on the GetCluster echo
	specAutoscaler *generated.AutoScalerConfig, // applied on the MR spec
) (*external, *mock_akuity_client.MockClient, *v1alpha1.Cluster) {
	t.Helper()
	e, mc := newExt(t, nil)

	mr := fixtures.CrossplaneManagedCluster.DeepCopy()
	mr.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	mr.Spec.ForProvider.ClusterSpec.Data.AutoscalerConfig = specAutoscaler

	getCluster := proto.Clone(fixtures.ArgocdCluster).(*argocdv1.Cluster)
	getData := getCluster.GetData()
	if getData == nil {
		getData = &argocdv1.ClusterData{}
		getCluster.Data = getData
	}
	getData.AutoscalerConfig = desired

	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(getCluster, nil).Times(1)
	// Export omits AutoscalerConfig server-side for non-`auto` clusters
	// (the lateInit-vs-Export gap); the fixture's ExportedCluster has
	// no AutoscalerConfig in any of these scenarios. The drift override
	// in Observe is what makes the comparator behave correctly.
	mc.EXPECT().ExportInstanceByID(ctx, fixtures.InstanceID).
		Return(&argocdv1.ExportInstanceResponse{Clusters: []*structpb.Struct{fixtures.ExportedCluster}}, nil).Times(1)

	return e, mc, mr
}

// TestObserve_AutoscalerConfig_UserSetServerMissing covers the
// principal Issue 2 symptom: user populates spec.data.autoscalerConfig,
// the platform has not yet stamped it (Get echoes nil because Apply
// hasn't propagated the value or the cluster size doesn't qualify).
// The previous normalizePtrField path collapsed desired to nil and
// reported up-to-date, silently dropping user intent. The fix routes
// drift through the Get-based override so this case fires drift and
// Apply runs.
func TestObserve_AutoscalerConfig_UserSetServerMissing(t *testing.T) {
	specAS := &generated.AutoScalerConfig{
		ApplicationController: &generated.AppControllerAutoScalingConfig{
			ResourceMinimum: &generated.Resources{Mem: "256Mi", Cpu: "100m"},
		},
	}
	e, _, mr := observeFixtureWithAutoscaler(t, nil, specAS)
	resp, err := e.Observe(ctx, mr)
	require.NoError(t, err)
	assert.False(t, resp.ResourceUpToDate,
		"user-set AutoscalerConfig with server-side nil must surface as drift, not silent collapse")
}

// TestObserve_AutoscalerConfig_DesiredNilServerStamped covers the
// reverse direction: user leaves the field unset, the platform
// stamps a default. The comparator must NOT fire drift on the
// server-stamped default; Apply with desired=nil would zero out the
// server's stamp.
func TestObserve_AutoscalerConfig_DesiredNilServerStamped(t *testing.T) {
	getAS := &argocdv1.AutoScalerConfig{
		ApplicationController: &argocdv1.AppControllerAutoScalingConfig{
			ResourceMinimum: &argocdv1.Resources{Mem: "256Mi", Cpu: "100m"},
		},
	}
	e, _, mr := observeFixtureWithAutoscaler(t, getAS, nil)
	resp, err := e.Observe(ctx, mr)
	require.NoError(t, err)
	assert.True(t, resp.ResourceUpToDate,
		"server-stamped default with desired-nil must adopt observed, not flap drift")
}

// TestObserve_AutoscalerConfig_BothPopulatedEqual covers the
// agree case: user pinned a value and the platform agrees. No drift.
func TestObserve_AutoscalerConfig_BothPopulatedEqual(t *testing.T) {
	getAS := &argocdv1.AutoScalerConfig{
		ApplicationController: &argocdv1.AppControllerAutoScalingConfig{
			ResourceMinimum: &argocdv1.Resources{Mem: "256Mi", Cpu: "100m"},
		},
	}
	specAS := &generated.AutoScalerConfig{
		ApplicationController: &generated.AppControllerAutoScalingConfig{
			ResourceMinimum: &generated.Resources{Mem: "256Mi", Cpu: "100m"},
		},
	}
	e, _, mr := observeFixtureWithAutoscaler(t, getAS, specAS)
	resp, err := e.Observe(ctx, mr)
	require.NoError(t, err)
	assert.True(t, resp.ResourceUpToDate)
}

// TestObserve_AutoscalerConfig_BothPopulatedDifferent covers the
// disagree case: user pinned a different value than the platform.
// Drift fires so Apply runs and the user's value wins (or the
// platform rejects).
func TestObserve_AutoscalerConfig_BothPopulatedDifferent(t *testing.T) {
	getAS := &argocdv1.AutoScalerConfig{
		ApplicationController: &argocdv1.AppControllerAutoScalingConfig{
			ResourceMinimum: &argocdv1.Resources{Mem: "256Mi", Cpu: "100m"},
		},
	}
	specAS := &generated.AutoScalerConfig{
		ApplicationController: &generated.AppControllerAutoScalingConfig{
			ResourceMinimum: &generated.Resources{Mem: "1Gi", Cpu: "500m"},
		},
	}
	e, _, mr := observeFixtureWithAutoscaler(t, getAS, specAS)
	resp, err := e.Observe(ctx, mr)
	require.NoError(t, err)
	assert.False(t, resp.ResourceUpToDate)
}

// observeFixtureWithPodInheritMetadata mirrors observeFixtureWithAutoscaler
// for the PodInheritMetadata pointer-field case: GetCluster carries the
// platform-stored value while ExportInstance omits it (same wire-shape
// gap as AutoscalerConfig / Compatibility / ArgocdNotificationsSettings).
func observeFixtureWithPodInheritMetadata(
	t *testing.T,
	getValue *bool,
	specValue *bool,
) (*external, *mock_akuity_client.MockClient, *v1alpha1.Cluster) {
	t.Helper()
	e, mc := newExt(t, nil)

	mr := fixtures.CrossplaneManagedCluster.DeepCopy()
	mr.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	mr.Spec.ForProvider.ClusterSpec.Data.PodInheritMetadata = specValue

	getCluster := proto.Clone(fixtures.ArgocdCluster).(*argocdv1.Cluster)
	getData := getCluster.GetData()
	if getData == nil {
		getData = &argocdv1.ClusterData{}
		getCluster.Data = getData
	}
	getData.PodInheritMetadata = getValue

	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(getCluster, nil).Times(1)
	mc.EXPECT().ExportInstanceByID(ctx, fixtures.InstanceID).
		Return(&argocdv1.ExportInstanceResponse{Clusters: []*structpb.Struct{fixtures.ExportedCluster}}, nil).Times(1)

	return e, mc, mr
}

// TestObserve_PodInheritMetadata_DesiredTrueServerEcho covers the
// reported drift-flap: user pins data.podInheritMetadata=true,
// GetCluster echoes true, but ExportInstance omits the field. Without
// the GetCluster-based override the comparator sees desired=&true vs
// driftTarget=nil and re-Applies on every poll.
func TestObserve_PodInheritMetadata_DesiredTrueServerEcho(t *testing.T) {
	v := true
	e, _, mr := observeFixtureWithPodInheritMetadata(t, &v, &v)
	resp, err := e.Observe(ctx, mr)
	require.NoError(t, err)
	assert.True(t, resp.ResourceUpToDate,
		"user-pinned PodInheritMetadata=true that GetCluster echoes must not flap drift when Export omits the field")
}

// TestObserve_PodInheritMetadata_DesiredNilServerStamped covers the
// reverse direction: user leaves the field unset, the platform stamps
// a value. Apply with desired=nil would clear the platform's stamp;
// driftTarget must adopt the observed value so no Apply fires.
func TestObserve_PodInheritMetadata_DesiredNilServerStamped(t *testing.T) {
	v := true
	e, _, mr := observeFixtureWithPodInheritMetadata(t, &v, nil)
	resp, err := e.Observe(ctx, mr)
	require.NoError(t, err)
	assert.True(t, resp.ResourceUpToDate,
		"server-stamped PodInheritMetadata with desired-nil must adopt observed, not flap drift")
}

// TestObserve_PodInheritMetadata_DesiredTrueServerFalse covers the
// disagree case: user pinned true, the platform actually has false.
// Drift must fire so Apply runs.
func TestObserve_PodInheritMetadata_DesiredTrueServerFalse(t *testing.T) {
	tr, fa := true, false
	e, _, mr := observeFixtureWithPodInheritMetadata(t, &fa, &tr)
	resp, err := e.Observe(ctx, mr)
	require.NoError(t, err)
	assert.False(t, resp.ResourceUpToDate,
		"user-pinned PodInheritMetadata=true with GetCluster=false must surface as drift")
}

// observeFixtureWithDatadogEksAddon mirrors the pattern for the
// DatadogAnnotationsEnabled / EksAddonEnabled tri-state fields. Both
// sit on ClusterData with the same Export-omits-vs-Get-echoes shape
// gap as PodInheritMetadata; the Export wire form additionally surfaces
// `&false` when the user is silent, so per-poll wasteful Apply fires
// without a normalize entry to fold the user-silent side onto observed.
func observeFixtureWithDatadogEksAddon(
	t *testing.T,
	getDatadog, getEksAddon *bool,
	specDatadog, specEksAddon *bool,
) (*external, *mock_akuity_client.MockClient, *v1alpha1.Cluster) {
	t.Helper()
	e, mc := newExt(t, nil)

	mr := fixtures.CrossplaneManagedCluster.DeepCopy()
	mr.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.ClusterName,
		},
	}
	mr.Spec.ForProvider.ClusterSpec.Data.DatadogAnnotationsEnabled = specDatadog
	mr.Spec.ForProvider.ClusterSpec.Data.EksAddonEnabled = specEksAddon

	getCluster := proto.Clone(fixtures.ArgocdCluster).(*argocdv1.Cluster)
	getData := getCluster.GetData()
	if getData == nil {
		getData = &argocdv1.ClusterData{}
		getCluster.Data = getData
	}
	getData.DatadogAnnotationsEnabled = getDatadog
	getData.EksAddonEnabled = getEksAddon

	mc.EXPECT().GetCluster(ctx, fixtures.InstanceID, fixtures.ClusterName).
		Return(getCluster, nil).Times(1)
	mc.EXPECT().ExportInstanceByID(ctx, fixtures.InstanceID).
		Return(&argocdv1.ExportInstanceResponse{Clusters: []*structpb.Struct{fixtures.ExportedCluster}}, nil).Times(1)

	return e, mc, mr
}

// TestObserve_DatadogEksAddon_DesiredNilServerStamped covers the
// reported per-poll wasteful Apply: user is silent, the platform
// stamps DatadogAnnotationsEnabled=&false / EksAddonEnabled=&false on
// the wire (proto3 oneof), and without the Normalize entries the
// comparator flapped drift on every reconcile.
func TestObserve_DatadogEksAddon_DesiredNilServerStamped(t *testing.T) {
	fa := false
	e, _, mr := observeFixtureWithDatadogEksAddon(t, &fa, &fa, nil, nil)
	resp, err := e.Observe(ctx, mr)
	require.NoError(t, err)
	assert.True(t, resp.ResourceUpToDate,
		"server-stamped Datadog/EksAddon defaults with desired-nil must adopt observed, not flap drift")
}

// TestObserve_DatadogEksAddon_DesiredTrueServerFalse covers the
// disagree case: user pinned true, the platform has the proto3 default.
// Drift must fire so Apply runs.
func TestObserve_DatadogEksAddon_DesiredTrueServerFalse(t *testing.T) {
	tr, fa := true, false
	e, _, mr := observeFixtureWithDatadogEksAddon(t, &fa, &fa, &tr, &tr)
	resp, err := e.Observe(ctx, mr)
	require.NoError(t, err)
	assert.False(t, resp.ResourceUpToDate,
		"user-pinned Datadog/EksAddon=true with platform=false must surface as drift")
}
