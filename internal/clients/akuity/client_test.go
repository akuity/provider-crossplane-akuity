package akuity_test

import (
	"context"
	"errors"
	"testing"

	reconv1 "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"

	"github.com/akuity/api-client-go/pkg/api/gateway/accesscontrol"
	gwoption "github.com/akuity/api-client-go/pkg/api/gateway/option"
	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	orgcv1 "github.com/akuity/api-client-go/pkg/api/gen/organization/v1"
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
)

var (
	organizationID = "test-org-id"
	instanceID     = "test-instance-id"
	instanceName   = "test-instance-name"
	workspaceID    = "test-workspace-id"
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

func expectGetInstanceByID(mockGatewayClient *mock_akuity_client.MockArgoCDServiceGatewayClient, id string) {
	expectGetInstanceByIDWithWorkspace(mockGatewayClient, id, "")
}

func expectGetInstanceByIDWithWorkspace(mockGatewayClient *mock_akuity_client.MockArgoCDServiceGatewayClient, id, workspace string) {
	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_ID,
		Id:             id,
	}).Return(&argocdv1.GetInstanceResponse{
		Instance: &argocdv1.Instance{
			Id:          id,
			WorkspaceId: workspace,
		},
	}, nil).AnyTimes()
}

func expectListWorkspaces(mockOrgGatewayClient *mock_akuity_client.MockOrganizationServiceGatewayClient, workspaces ...*orgcv1.Workspace) {
	mockOrgGatewayClient.EXPECT().ListWorkspaces(authCtx, &orgcv1.ListWorkspacesRequest{
		OrganizationId: organizationID,
	}).Return(&orgcv1.ListWorkspacesResponse{Workspaces: workspaces}, nil).Times(1)
}

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
			_, err := akuity.NewClient(tc.organizationID, tc.apiKeyID, tc.apiKeySecret, gatewayClient, nil, nil)
			require.EqualError(t, err, tc.expectedErrStr)
		})
	}
}

func TestNewClient(t *testing.T) {
	gatewayClient := argocdv1.NewArgoCDServiceGatewayClient(gwoption.NewClient("fake", false))
	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, gatewayClient, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestGetCluster(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockResponse := &argocdv1.GetInstanceClusterResponse{
		Cluster: &argocdv1.Cluster{},
	}
	expectGetInstanceByID(mockGatewayClient, instanceID)

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(mockResponse, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.NoError(t, err)
	assert.NotNil(t, cluster)
}

func TestGetCluster_UsesInstanceWorkspaceID(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	expectGetInstanceByIDWithWorkspace(mockGatewayClient, instanceID, workspaceID)

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		WorkspaceId:    workspaceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(&argocdv1.GetInstanceClusterResponse{
		Cluster: &argocdv1.Cluster{},
	}, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.NoError(t, err)
	assert.NotNil(t, cluster)
}

func TestGetCluster_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	expectGetInstanceByID(mockGatewayClient, instanceID)

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Nil(t, cluster)
}

func TestGetCluster_StatusNotFound(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	expectGetInstanceByID(mockGatewayClient, instanceID)

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, statusNotFound).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.True(t, reason.IsNotFound(err))
	assert.Nil(t, cluster)
}

func TestGetCluster_NilResponse(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	expectGetInstanceByID(mockGatewayClient, instanceID)

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	cluster, err := client.GetCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Nil(t, cluster)
}

func TestGetCluster_NilCluster(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	expectGetInstanceByID(mockGatewayClient, instanceID)

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(&argocdv1.GetInstanceClusterResponse{}, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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
	expectGetInstanceByID(mockGatewayClient, instanceID)

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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.NoError(t, err)
	assert.Equal(t, "test-manifests", manifests)
}

func TestGetClusterManifests_GetClusterErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	expectGetInstanceByID(mockGatewayClient, instanceID)

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, errors.New("fake")).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Empty(t, manifests)
}

func TestGetClusterManifests_ClusterNotReconciledErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	expectGetInstanceByID(mockGatewayClient, instanceID)

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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Empty(t, manifests)
}

func TestGetClusterManifests_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	expectGetInstanceByID(mockGatewayClient, instanceID)

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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Empty(t, manifests)
}

func TestGetClusterManifests_ResponseErrChanErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockErrChan := make(chan error, 1)
	mockErrChan <- errors.New("fake")
	expectGetInstanceByID(mockGatewayClient, instanceID)

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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	manifests, err := client.GetClusterManifests(ctx, instanceID, clusterName)
	require.Error(t, err)
	assert.Empty(t, manifests)
}

func TestDeleteCluster(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockCluster := &argocdv1.Cluster{
		Id: clusterID,
	}
	expectGetInstanceByID(mockGatewayClient, instanceID)

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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	err = client.DeleteCluster(ctx, instanceID, clusterName)
	require.NoError(t, err)
}

func TestDeleteCluster_GetClusterErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	expectGetInstanceByID(mockGatewayClient, instanceID)

	mockGatewayClient.EXPECT().GetInstanceCluster(authCtx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             clusterName,
	}).Return(nil, errFake).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	err = client.DeleteCluster(ctx, instanceID, clusterName)
	require.Error(t, err)
}

func TestDeleteCluster_ClientErr(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	mockCluster := &argocdv1.Cluster{
		Id: clusterID,
	}
	expectGetInstanceByID(mockGatewayClient, instanceID)

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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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
	expectGetInstanceByID(mockGatewayClient, instanceID)

	mockGatewayClient.EXPECT().ApplyInstance(authCtx, mockRequest).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	err = client.ApplyInstance(ctx, mockRequest)
	require.NoError(t, err)
}

func TestApplyInstance_IDRequestWorkspaceWins(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(ctrl)
	mockOrgGatewayClient := mock_akuity_client.NewMockOrganizationServiceGatewayClient(ctrl)
	callerRequest := &argocdv1.ApplyInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_ID,
		Id:             instanceID,
		WorkspaceId:    "workspace-name",
	}
	expected := &argocdv1.ApplyInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_ID,
		Id:             instanceID,
		WorkspaceId:    workspaceID,
	}
	expectListWorkspaces(mockOrgGatewayClient, &orgcv1.Workspace{
		Id:   workspaceID,
		Name: "workspace-name",
	})
	mockGatewayClient.EXPECT().ApplyInstance(authCtx, expected).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, mockOrgGatewayClient)
	require.NoError(t, err)

	err = client.ApplyInstance(ctx, callerRequest)
	require.NoError(t, err)
	assert.Equal(t, workspaceID, callerRequest.GetWorkspaceId())
}

func TestApplyInstance_NameRequestDefaultsWorkspaceOnCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(ctrl)
	mockOrgGatewayClient := mock_akuity_client.NewMockOrganizationServiceGatewayClient(ctrl)
	callerRequest := &argocdv1.ApplyInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}
	expected := &argocdv1.ApplyInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
		WorkspaceId:    workspaceID,
	}
	mockGatewayClient.EXPECT().GetInstance(authCtx, &argocdv1.GetInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
	}).Return(nil, statusNotFound).Times(1)
	expectListWorkspaces(mockOrgGatewayClient, &orgcv1.Workspace{
		Id:        workspaceID,
		Name:      "default",
		IsDefault: true,
	})
	mockGatewayClient.EXPECT().ApplyInstance(authCtx, expected).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, mockOrgGatewayClient)
	require.NoError(t, err)

	err = client.ApplyInstance(ctx, callerRequest)
	require.NoError(t, err)
	assert.Equal(t, workspaceID, callerRequest.GetWorkspaceId())
}

func TestApplyInstance_WorkspaceLookupNotFoundIsTerminal(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(ctrl)
	mockOrgGatewayClient := mock_akuity_client.NewMockOrganizationServiceGatewayClient(ctrl)
	callerRequest := &argocdv1.ApplyInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             instanceName,
		WorkspaceId:    "missing-workspace",
	}
	expectListWorkspaces(mockOrgGatewayClient, &orgcv1.Workspace{
		Id:        workspaceID,
		Name:      "default",
		IsDefault: true,
	})

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, mockOrgGatewayClient)
	require.NoError(t, err)

	err = client.ApplyInstance(ctx, callerRequest)
	require.Error(t, err)
	assert.True(t, reason.IsTerminal(err))
}

// TestApplyInstance_FillsOrganizationId guards the controller
// contract: callers (InstanceIpAllowList) construct the request
// without setting OrganizationId, relying on the client wrapper to
// inject it from the ProviderConfig-bound organization.
func TestApplyInstance_FillsOrganizationId(t *testing.T) {
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(gomock.NewController(t))
	callerRequest := &argocdv1.ApplyInstanceRequest{
		// OrganizationId left empty on purpose.
		IdType: idv1.Type_NAME,
		Id:     "my-instance",
	}
	expected := &argocdv1.ApplyInstanceRequest{
		OrganizationId: organizationID,
		IdType:         idv1.Type_NAME,
		Id:             "my-instance",
	}

	mockGatewayClient.EXPECT().ApplyInstance(authCtx, expected).Return(nil, nil).Times(1)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	err = client.ApplyInstance(ctx, callerRequest)
	require.NoError(t, err)
	assert.Equal(t, organizationID, callerRequest.GetOrganizationId(), "ApplyInstance must auto-fill OrganizationId on the outgoing request")
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
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

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, nil)
	require.NoError(t, err)

	err = client.DeleteInstance(ctx, instanceName)
	require.Error(t, err)
}

func TestResolveWorkspace_MatchesIDBeforeName(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(ctrl)
	mockOrgGatewayClient := mock_akuity_client.NewMockOrganizationServiceGatewayClient(ctrl)
	idMatch := &orgcv1.Workspace{
		Id:   "workspace-ref",
		Name: "id-match",
	}
	nameMatch := &orgcv1.Workspace{
		Id:   "other-id",
		Name: "workspace-ref",
	}
	expectListWorkspaces(mockOrgGatewayClient, nameMatch, idMatch)

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, mockOrgGatewayClient)
	require.NoError(t, err)

	workspace, err := client.ResolveWorkspace(ctx, "workspace-ref")
	require.NoError(t, err)
	assert.Equal(t, idMatch.GetId(), workspace.GetId())
}

func TestResolveWorkspace_Default(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockGatewayClient := mock_akuity_client.NewMockArgoCDServiceGatewayClient(ctrl)
	mockOrgGatewayClient := mock_akuity_client.NewMockOrganizationServiceGatewayClient(ctrl)
	expectListWorkspaces(mockOrgGatewayClient, &orgcv1.Workspace{
		Id:        workspaceID,
		Name:      "default",
		IsDefault: true,
	})

	client, err := akuity.NewClient(organizationID, apiKeyID, apiKeySecret, mockGatewayClient, nil, mockOrgGatewayClient)
	require.NoError(t, err)

	workspace, err := client.ResolveWorkspace(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, workspaceID, workspace.GetId())
}
