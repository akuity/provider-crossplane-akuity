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

package instance_test

import (
	"context"
	"testing"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	mock_akuity_client "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/instance"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
)

var (
	ctx            = context.TODO()
	organizationID = "organization-id"
)

func TestCreate(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	unmockedAkuityClient, err := akuity.NewClient(organizationID, "fake-api-key", "fake-api-secret", mockGatewayClient)
	require.NoError(t, err)

	applyInstanceRequest, err := unmockedAkuityClient.BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance)
	require.NoError(t, err)

	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	mockAkuityClient.EXPECT().BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance).
		Return(applyInstanceRequest, nil).Times(1)

	mockAkuityClient.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(nil).Times(1)

	resp, err := client.Create(ctx, &fixtures.CrossplaneManagedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_NotInstanceErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	resp, err := client.Create(ctx, &v1alpha1.Cluster{})
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_BuildRequestErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	mockAkuityClient.EXPECT().BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance).
		Return(nil, errors.New("fake")).Times(1)

	resp, err := client.Create(ctx, &fixtures.CrossplaneManagedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	unmockedAkuityClient, err := akuity.NewClient(organizationID, "fake-api-key", "fake-api-secret", mockGatewayClient)
	require.NoError(t, err)

	applyInstanceRequest, err := unmockedAkuityClient.BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance)
	require.NoError(t, err)

	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	mockAkuityClient.EXPECT().BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance).
		Return(applyInstanceRequest, nil).Times(1)

	mockAkuityClient.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(errors.New("fake")).Times(1)

	resp, err := client.Create(ctx, &fixtures.CrossplaneManagedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestUpdate(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	unmockedAkuityClient, err := akuity.NewClient(organizationID, "fake-api-key", "fake-api-secret", mockGatewayClient)
	require.NoError(t, err)

	applyInstanceRequest, err := unmockedAkuityClient.BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance)
	require.NoError(t, err)

	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	mockAkuityClient.EXPECT().BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance).
		Return(applyInstanceRequest, nil).Times(1)

	mockAkuityClient.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(nil).Times(1)

	resp, err := client.Update(ctx, &fixtures.CrossplaneManagedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestUpdate_NotInstanceErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	resp, err := client.Update(ctx, &v1alpha1.Cluster{})
	require.Error(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestUpdate_BuildRequestErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	mockAkuityClient.EXPECT().BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance).
		Return(nil, errors.New("fake")).Times(1)

	resp, err := client.Update(ctx, &fixtures.CrossplaneManagedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestUpdate_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	unmockedAkuityClient, err := akuity.NewClient(organizationID, "fake-api-key", "fake-api-secret", mockGatewayClient)
	require.NoError(t, err)

	applyInstanceRequest, err := unmockedAkuityClient.BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance)
	require.NoError(t, err)

	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	mockAkuityClient.EXPECT().BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance).
		Return(applyInstanceRequest, nil).Times(1)

	mockAkuityClient.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(errors.New("fake")).Times(1)

	resp, err := client.Update(ctx, &fixtures.CrossplaneManagedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestDelete(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mockAkuityClient.EXPECT().DeleteInstance(ctx, fixtures.InstanceName).Return(nil).Times(1)

	err := client.Delete(ctx, &managedInstance)
	require.NoError(t, err)
}

func TestDelete_NotInstanceErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	err := client.Delete(ctx, &v1alpha1.Cluster{})
	require.Error(t, err)
}

func TestDelete_EmptyExternalName(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	err := client.Delete(ctx, &v1alpha1.Instance{})
	require.NoError(t, err)
}

func TestDelete_ClientErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mockAkuityClient.EXPECT().DeleteInstance(ctx, fixtures.InstanceName).Return(errors.New("fake")).Times(1)

	err := client.Delete(ctx, &managedInstance)
	require.Error(t, err)
}
func TestObserve_NotInstanceErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())

	_, err := client.Observe(ctx, &v1alpha1.Cluster{})
	require.Error(t, err)
}

func TestObserve_EmptyExternalName(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))
	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	resp, err := client.Observe(ctx, &v1alpha1.Instance{})
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_GetInstanceNotFoundErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mockAkuityClient.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(nil, reason.AsNotFound(errors.New("not found"))).Times(1)

	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	resp, err := client.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_GetInstanceErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mockAkuityClient.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(nil, errors.New("fake")).Times(1)

	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	resp, err := client.Observe(ctx, &managedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
	assert.Equal(t, xpv1.ReasonReconcileError, managedInstance.Status.Conditions[0].Reason)
}

func TestObserve_ExportInstanceErr(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mockAkuityClient.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mockAkuityClient.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(nil, errors.New("fake")).Times(1)

	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	resp, err := client.Observe(ctx, &managedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
	assert.Equal(t, xpv1.ReasonReconcileError, managedInstance.Status.Conditions[0].Reason)
}

func TestObserve_HealthStatusNotHealthy(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))

	akuityInstance := fixtures.AkuityInstance
	akuityInstance.HealthStatus = &health.Status{
		Code:    health.StatusCode_STATUS_CODE_DEGRADED,
		Message: "Instance is degraded",
	}

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mockAkuityClient.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mockAkuityClient.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	_, err := client.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, xpv1.Unavailable().Reason, managedInstance.Status.Conditions[0].Reason)
}

func TestObserve_HealthStatusHealthy(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))

	akuityInstance := fixtures.AkuityInstance
	akuityInstance.HealthStatus = &health.Status{
		Code:    health.StatusCode_STATUS_CODE_HEALTHY,
		Message: "Instance is healthy",
	}

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mockAkuityClient.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mockAkuityClient.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	_, err := client.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, xpv1.Available().Reason, managedInstance.Status.Conditions[0].Reason)
}

func TestObserve_InstanceUpToDate(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mockAkuityClient.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mockAkuityClient.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	resp, err := client.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, resp)
}

func TestObserve_InstanceNotUpToDate(t *testing.T) {
	mockAkuityClient := mock_akuity_client.NewMockClient(gomock.NewController(t))

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}
	managedInstance.Spec.ForProvider.ArgoCD.Spec.Description = "new-description"

	mockAkuityClient.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mockAkuityClient.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	client := instance.NewExternal(mockAkuityClient, logging.NewNopLogger())
	resp, err := client.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, resp)
}
