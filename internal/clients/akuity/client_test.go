package akuity_test

import (
	"context"
	"errors"
	"testing"

	reconv1 "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"

	"github.com/akuity/api-client-go/pkg/api/gateway/accesscontrol"
	gwoption "github.com/akuity/api-client-go/pkg/api/gateway/option"
	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	httpctx "github.com/akuity/grpc-gateway-client/pkg/http/context"
	"go.uber.org/mock/gomock"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	mock_akuity_client "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/protobuf"
)

var (
	organizationID = "test-org-id"
	instanceID     = "test-instance-id"
	instanceName   = "test-instance-name"
	clusterID      = "test-cluster-id"
	clusterName    = "test-cluster-name"
	apiKeyID       = "test-api-key-id"
	apiKeySecret   = "test-api-key-secret"
	credentials    = accesscontrol.NewAPIKeyCredential(apiKeyID, apiKeySecret)
	ctx            = context.TODO()
	authCtx        = httpctx.SetAuthorizationHeader(ctx, credentials.Scheme(), credentials.Credential())
	statusNotFound = status.New(codes.NotFound, "not found").Err()
	errFake        = errors.New("fake")
)

func TestNewClient_Err(t *testing.T) {
	gatewayClient := argocdv1.NewArgoCDServiceGatewayClient(gwoption.NewClient("fake", false))

	cases := map[string]struct {
		organizationID string
		apiKeyID       string
		apiKeySecret   string
		expectedErrStr string
	}{
		"empty-organization-id": {
			organizationID: "",
			expectedErrStr: "organization ID must not be empty",
		},
		"empty-api-key-id": {
			organizationID: organizationID,
			apiKeyID:       "",
			expectedErrStr: "API key ID must not be empty",
		},
		"empty-api-key-secret": {
			organizationID: organizationID,
			apiKeyID:       apiKeyID,
			expectedErrStr: "API key secret must not be empty",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := akuity.NewClient(tc.organizationID, tc.apiKeyID, tc.apiKeySecret, gatewayClient)
			require.EqualError(t, err, tc.expectedErrStr)
		})
	}
}

func TestNewClient(t *testing.T) {
	gatewayClient := argocdv1.NewArgoCDServiceGatewayClient(gwoption.NewClient("fake", false))
	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, gatewayClient)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestGetCluster(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockResponse := &argocdv1.GetInstanceClusterResponse{
		Cluster: &argocdv1.Cluster{},
	}

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(mockResponse, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.NoError(t, err)
	assert.NotNil(t, cluster)
}

func TestGetCluster_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Nil(t, cluster)
}

func TestGetCluster_StatusNotFound(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, statusNotFound).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.True(t, reason.IsNotFound(err))
	assert.Nil(t, cluster)
}

func TestGetCluster_NilResponse(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Nil(t, cluster)
}

func TestGetCluster_NilCluster(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(&argocdv1.GetInstanceClusterResponse{}, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Nil(t, cluster)
}

func TestGetClusterManifests(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockResponseChan := make(chan *httpbody.HttpBody, 1)
	mockResponseChan <- &httpbody.HttpBody{
		Data: []byte("test-manifests"),
	}

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(&argocdv1.GetInstanceClusterResponse{
		Cluster: &argocdv1.Cluster{
			Id: clusterID,
			ReconciliationStatus: &reconv1.Status{
				Code: reconv1.StatusCode_STATUS_CODE_SUCCESSFUL,
			},
		},
	}, nil).Times(1)

	mockGatewayClient.EXPECT().GetInstanceClusterManifests(authCtx, &argocdv1.GetInstanceClusterManifestsRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		Id:             clusterID,
	}).Return(mockResponseChan, nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.NoError(t, err)
	assert.Equal(t, "test-manifests", manifests)
}

func TestGetClusterManifests_GetClusterErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, errors.New("fake")).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Empty(t, manifests)
}

func TestGetClusterManifests_ClusterNotReconciledErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(&argocdv1.GetInstanceClusterResponse{
		Cluster: &argocdv1.Cluster{
			Id: clusterID,
			ReconciliationStatus: &reconv1.Status{
				Code: reconv1.StatusCode_STATUS_CODE_PROGRESSING,
			},
		},
	}, nil).Times(5)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Empty(t, manifests)
}

func TestGetClusterManifests_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(&argocdv1.GetInstanceClusterResponse{
		Cluster: &argocdv1.Cluster{
			Id: clusterID,
			ReconciliationStatus: &reconv1.Status{
				Code: reconv1.StatusCode_STATUS_CODE_SUCCESSFUL,
			},
		},
	}, nil).Times(1)

	mockGatewayClient.EXPECT().GetInstanceClusterManifests(authCtx, &argocdv1.GetInstanceClusterManifestsRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		Id:             clusterID,
	}).Return(nil, nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Empty(t, manifests)
}

func TestGetClusterManifests_ResponseErrChanErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockErrChan := make(chan error, 1)
	mockErrChan <- errors.New("fake")

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(&argocdv1.GetInstanceClusterResponse{
		Cluster: &argocdv1.Cluster{
			Id: clusterID,
			ReconciliationStatus: &reconv1.Status{
				Code: reconv1.StatusCode_STATUS_CODE_SUCCESSFUL,
			},
		},
	}, nil).Times(1)

	mockGatewayClient.EXPECT().GetInstanceClusterManifests(authCtx, &argocdv1.GetInstanceClusterManifestsRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		Id:             clusterID,
	}).Return(nil, mockErrChan, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Empty(t, manifests)
}

func TestApplyCluster(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	cluster := akuitytypes.Cluster{}
	clusterPB, _ := protobuf.MarshalObjectToProtobufStruct(cluster)

	request := &argocdv1.ApplyInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_ID,
		Id:             instanceID,
		Clusters:       []*structpb.Struct{clusterPB},
	}

	mockGatewayClient.EXPECT().ApplyInstance(authCtx, request).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	err = client.ApplyCluster(ctx, instanceID, cluster)
	require.NoError(t, err)
}
func TestDeleteCluster(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockCluster := &argocdv1.Cluster{
		Id: clusterID,
	}

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(&argocdv1.GetInstanceClusterResponse{
		Cluster: mockCluster,
	}, nil).Times(1)

	mockGatewayClient.EXPECT().DeleteInstanceCluster(authCtx, &argocdv1.DeleteInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		Id:             mockCluster.GetId(),
	}).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	err = client.DeleteCluster(ctx, instanceID, clusterName)
	require.NoError(t, err)
}

func TestDeleteCluster_GetClusterErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	err = client.DeleteCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
}

func TestDeleteCluster_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockCluster := &argocdv1.Cluster{
		Id: clusterID,
	}

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(&argocdv1.GetInstanceClusterResponse{
		Cluster: mockCluster,
	}, nil).Times(1)

	mockGatewayClient.EXPECT().DeleteInstanceCluster(authCtx, &argocdv1.DeleteInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		Id:             mockCluster.GetId(),
	}).Return(nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	err = client.DeleteCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
}

func TestGetInstance(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockResponse := &argocdv1.GetInstanceResponse{
		Instance: &argocdv1.Instance{},
	}

	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(mockResponse, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	instance, err := client.GetInstance(ctx, instanceName)
	require.NoError(t, err)
	assert.NotNil(t, instance)
}

func TestGetInstance_StatusNotFound(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(nil, statusNotFound).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	instance, err := client.GetInstance(ctx, instanceName)
	require.Error(t, err)
	assert.True(t, reason.IsNotFound(err))
	assert.Nil(t, instance)
}

func TestGetInstance_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	instance, err := client.GetInstance(ctx, instanceName)
	require.Error(t, err)
	assert.Nil(t, instance)
}

func TestGetInstance_NilResponse(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	instance, err := client.GetInstance(ctx, instanceName)
	require.Error(t, err)
	assert.Nil(t, instance)
}

func TestGetInstance_NilInstance(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockResponse := &argocdv1.GetInstanceResponse{
		Instance: nil,
	}

	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(mockResponse, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	instance, err := client.GetInstance(ctx, instanceName)
	require.Error(t, err)
	assert.Nil(t, instance)
}

func TestExportInstance(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockResponse := &argocdv1.ExportInstanceResponse{}

	mockGatewayClient.EXPECT().ExportInstance(authCtx, &argocdv1.ExportInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(mockResponse, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	resp, err := client.ExportInstance(ctx, instanceName)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestExportInstance_statusNotFound(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().ExportInstance(authCtx, &argocdv1.ExportInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(nil, statusNotFound).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	resp, err := client.ExportInstance(ctx, instanceName)
	require.Error(t, err)
	assert.True(t, reason.IsNotFound(err))
	assert.Nil(t, resp)
}

func TestExportInstance_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().ExportInstance(authCtx, &argocdv1.ExportInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	resp, err := client.ExportInstance(ctx, instanceName)
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestExportInstance_NilResponse(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().ExportInstance(authCtx, &argocdv1.ExportInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	resp, err := client.ExportInstance(ctx, instanceName)
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestApplyInstance(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockRequest := &argocdv1.ApplyInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_ID,
		Id:             instanceID,
		Clusters:       []*structpb.Struct{},
	}

	mockGatewayClient.EXPECT().ApplyInstance(authCtx, mockRequest).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	err = client.ApplyInstance(ctx, mockRequest)
	require.NoError(t, err)
}

func TestDeleteInstance(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	response := &argocdv1.GetInstanceResponse{
		Instance: &argocdv1.Instance{
			Id: instanceID,
		},
	}

	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(response, nil).Times(1)

	mockGatewayClient.EXPECT().DeleteInstance(authCtx, &argocdv1.DeleteInstanceRequest{
		OrganizationId: organizationID,
		Id:             instanceID,
	}).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	err = client.DeleteInstance(ctx, instanceName)
	require.NoError(t, err)
}

func TestDeleteInstance_GetInstanceErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))

	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	err = client.DeleteInstance(ctx, instanceName)
	require.Error(t, err)
}

func TestDeleteInstance_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	response := &argocdv1.GetInstanceResponse{
		Instance: &argocdv1.Instance{
			Id: instanceID,
		},
	}

	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(response, nil).Times(1)

	mockGatewayClient.EXPECT().DeleteInstance(authCtx, &argocdv1.DeleteInstanceRequest{
		OrganizationId: organizationID,
		Id:             instanceID,
	}).Return(nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient)
	require.NoError(t, err)

	err = client.DeleteInstance(ctx, instanceName)
	require.Error(t, err)
}
