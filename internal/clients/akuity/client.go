package akuity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/avast/retry-go/v4"

	"github.com/akuity/api-client-go/pkg/api/gateway/accesscontrol"
	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	reconv1 "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"
	httpctx "github.com/akuity/grpc-gateway-client/pkg/http/context"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	"github.com/akuityio/provider-crossplane-akuity/internal/types"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/protobuf"
)

const (
	waitForReconciliationRetryAttempts = 5
)

type Client interface {
	GetCluster(ctx context.Context, instanceID string, name string) (*argocdv1.Cluster, error)
	GetClusterManifests(ctx context.Context, instanceID string, clusterName string) (string, error)
	ApplyCluster(ctx context.Context, instanceID string, cluster akuitytypes.Cluster) error
	DeleteCluster(ctx context.Context, instanceID string, name string) error
	GetInstance(ctx context.Context, name string) (*argocdv1.Instance, error)
	ExportInstance(ctx context.Context, name string) (*argocdv1.ExportInstanceResponse, error)
	ApplyInstance(ctx context.Context, request *argocdv1.ApplyInstanceRequest) error
	DeleteInstance(ctx context.Context, name string) error
	BuildApplyInstanceRequest(instance crossplanetypes.Instance) (*argocdv1.ApplyInstanceRequest, error)
}

type client struct {
	organizationID string
	credentials    accesscontrol.ClientCredential
	gatewayClient  argocdv1.ArgoCDServiceGatewayClient
}

func NewClient(organizationID string, apiKeyID string, apiKeySecret string, gatewayClient argocdv1.ArgoCDServiceGatewayClient) (Client, error) {
	if organizationID == "" {
		return client{}, errors.New("organization ID must not be empty")
	}

	if apiKeyID == "" {
		return client{}, errors.New("API key ID must not be empty")
	}

	if apiKeySecret == "" {
		return client{}, errors.New("API key secret must not be empty")
	}

	c := client{
		organizationID: organizationID,
		credentials:    accesscontrol.NewAPIKeyCredential(apiKeyID, apiKeySecret),
		gatewayClient:  gatewayClient,
	}

	return c, nil
}

func (c client) GetCluster(ctx context.Context, instanceID string, name string) (*argocdv1.Cluster, error) {
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.gatewayClient.GetInstanceCluster(ctx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: c.organizationID,
		InstanceId:     instanceID,
		IdType:         idv1.Type_NAME,
		Id:             name,
	})

	if err != nil {
		if e, ok := status.FromError(err); ok {
			if e.Code() == codes.NotFound {
				return nil, reason.AsNotFound(fmt.Errorf("could not get cluster %s from Akuity API, cluster was not found", name))
			}
		}
		return nil, fmt.Errorf("could not get cluster %s from Akuity API, error: %w", name, err)
	}

	if resp == nil {
		return nil, fmt.Errorf("could not get cluster %s from Akuity API, response was empty", name)
	}

	if resp.GetCluster() == nil {
		return nil, fmt.Errorf("could not get cluster %s from Akuity API, cluster is response was empty", name)
	}

	return resp.GetCluster(), nil
}

func (c client) GetClusterManifests(ctx context.Context, instanceID string, clusterName string) (string, error) {
	cluster, err := c.checkClusterReconciled(ctx, instanceID, clusterName)
	if err != nil {
		return "", err
	}

	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	respChan, errChan, err := c.gatewayClient.GetInstanceClusterManifests(ctx, &argocdv1.GetInstanceClusterManifestsRequest{
		OrganizationId: c.organizationID,
		InstanceId:     instanceID,
		Id:             cluster.GetId(),
	})

	if err != nil {
		return "", fmt.Errorf("could not get cluster manifests from Akuity API, error: %w", err)
	}

	return getManifestsFromResponse(respChan, errChan)
}

func (c client) ApplyCluster(ctx context.Context, instanceID string, cluster akuitytypes.Cluster) error {
	request, err := c.buildApplyClusterRequest(instanceID, cluster)
	if err != nil {
		return fmt.Errorf("could not build apply cluster request: %w", err)
	}

	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	_, err = c.gatewayClient.ApplyInstance(ctx, request)

	return err
}

func (c client) DeleteCluster(ctx context.Context, instanceID string, name string) error {
	cluster, err := c.GetCluster(ctx, instanceID, name)
	if err != nil {
		return fmt.Errorf("could not get cluster %s to delete, err: %w", name, err)
	}

	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	_, err = c.gatewayClient.DeleteInstanceCluster(ctx, &argocdv1.DeleteInstanceClusterRequest{
		OrganizationId: c.organizationID,
		InstanceId:     instanceID,
		Id:             cluster.GetId(),
	})

	if err != nil {
		return fmt.Errorf("could not delete cluster %s using Akuity API, error: %w", name, err)
	}

	return nil
}

func (c client) GetInstance(ctx context.Context, name string) (*argocdv1.Instance, error) {
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.gatewayClient.GetInstance(ctx, &argocdv1.GetInstanceRequest{
		OrganizationId: c.organizationID,
		IdType:         idv1.Type_NAME,
		Id:             name,
	})

	if err != nil {
		if e, ok := status.FromError(err); ok {
			if e.Code() == codes.NotFound {
				return nil, reason.AsNotFound(fmt.Errorf("could not get instance %s from Akuity API, instance was not found", name))
			}
		}
		return nil, fmt.Errorf("could not get instance %s from Akuity API, error: %w", name, err)
	}

	if resp == nil {
		return nil, fmt.Errorf("could not get instance %s from Akuity API, response was empty", name)
	}

	if resp.GetInstance() == nil {
		return nil, fmt.Errorf("could not get instance %s from Akuity API, instance is response was empty", name)
	}

	return resp.GetInstance(), nil
}

func (c client) ExportInstance(ctx context.Context, name string) (*argocdv1.ExportInstanceResponse, error) {
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.gatewayClient.ExportInstance(ctx, &argocdv1.ExportInstanceRequest{
		OrganizationId: c.organizationID,
		IdType:         idv1.Type_NAME,
		Id:             name,
	})

	if err != nil {
		if e, ok := status.FromError(err); ok {
			if e.Code() == codes.NotFound {
				return nil, reason.AsNotFound(fmt.Errorf("could not export instance %s from Akuity API, instance was not found", name))
			}
		}
		return nil, fmt.Errorf("could not export instance %s from Akuity API, error: %w", name, err)
	}

	if resp == nil {
		return nil, fmt.Errorf("could not export instance %s from Akuity API, response was empty", name)
	}

	return resp, nil
}

func (c client) ApplyInstance(ctx context.Context, request *argocdv1.ApplyInstanceRequest) error {
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	_, err := c.gatewayClient.ApplyInstance(ctx, request)

	return err
}

func (c client) DeleteInstance(ctx context.Context, name string) error {
	instance, err := c.GetInstance(ctx, name)
	if err != nil {
		return fmt.Errorf("could not get instance %s to delete, err: %w", name, err)
	}

	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	_, err = c.gatewayClient.DeleteInstance(ctx, &argocdv1.DeleteInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             instance.GetId(),
	})

	if err != nil {
		return fmt.Errorf("could not delete instance %s using Akuity API, error: %w", name, err)
	}

	return nil
}

func (c client) BuildApplyInstanceRequest(instance crossplanetypes.Instance) (*argocdv1.ApplyInstanceRequest, error) {
	argocdPB, err := types.CrossplaneToAkuityAPIArgoCD(instance.Spec.ForProvider.Name, instance.Spec.ForProvider.ArgoCD)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd spec to protobuf: %w", err)
	}

	argocdConfigMapPB, err := types.CrossplaneToAkuityAPIConfigMap(types.ARGOCD_CM_KEY, instance.Spec.ForProvider.ArgoCDConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd configmap to protobuf: %w", err)
	}

	argocdRbacConfigMapPB, err := types.CrossplaneToAkuityAPIConfigMap(types.ARGOCD_RBAC_CM_KEY, instance.Spec.ForProvider.ArgoCDRBACConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd rbac configmap to protobuf: %w", err)
	}

	argocdNotificationsConfigMapPB, err := types.CrossplaneToAkuityAPIConfigMap(types.ARGOCD_NOTIFICATIONS_CM_KEY, instance.Spec.ForProvider.ArgoCDNotificationsConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd notifications configmap to protobuf: %w", err)
	}

	argocdImageUpdaterConfigMapPB, err := types.CrossplaneToAkuityAPIConfigMap(types.ARGOCD_IMAGE_UPDATER_CM_KEY, instance.Spec.ForProvider.ArgoCDImageUpdaterConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd image updater configmap to protobuf: %w", err)
	}

	argocdImageUpdaterSshConfigMapPB, err := types.CrossplaneToAkuityAPIConfigMap(types.ARGOCD_IMAGE_UPDATER_SSH_CM_KEY, instance.Spec.ForProvider.ArgoCDImageUpdaterSSHConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd image updater ssh configmap to protobuf: %w", err)
	}

	argocdKnownHostsConfigMapPB, err := types.CrossplaneToAkuityAPIConfigMap(types.ARGOCD_SSH_KNOWN_HOSTS_CM_KEY, instance.Spec.ForProvider.ArgoCDSSHKnownHostsConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd known hosts configmap to protobuf: %w", err)
	}

	argocdTlsCertsConfigMapPB, err := types.CrossplaneToAkuityAPIConfigMap(types.ARGOCD_TLS_CERTS_CM_KEY, instance.Spec.ForProvider.ArgoCDTLSCertsConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd tls certs configmap to protobuf: %w", err)
	}

	configManagementPluginsPB, err := types.CrossplaneToAkuityAPIConfigManagementPlugins(instance.Spec.ForProvider.ConfigManagementPlugins)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd config management plugins to protobuf: %w", err)
	}

	request := &argocdv1.ApplyInstanceRequest{
		OrganizationId:            c.organizationID,
		IdType:                    idv1.Type_NAME,
		Id:                        instance.Spec.ForProvider.Name,
		Argocd:                    argocdPB,
		ArgocdConfigmap:           argocdConfigMapPB,
		ArgocdRbacConfigmap:       argocdRbacConfigMapPB,
		NotificationsConfigmap:    argocdNotificationsConfigMapPB,
		ImageUpdaterConfigmap:     argocdImageUpdaterConfigMapPB,
		ImageUpdaterSshConfigmap:  argocdImageUpdaterSshConfigMapPB,
		ArgocdKnownHostsConfigmap: argocdKnownHostsConfigMapPB,
		ArgocdTlsCertsConfigmap:   argocdTlsCertsConfigMapPB,
		ConfigManagementPlugins:   configManagementPluginsPB,
	}

	return request, nil
}

func (c client) buildApplyClusterRequest(instanceID string, cluster akuitytypes.Cluster) (*argocdv1.ApplyInstanceRequest, error) {
	clusterPB, err := protobuf.MarshalObjectToProtobufStruct(cluster)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s cluster to protobuf struct: %w", cluster.Name, err)
	}

	return &argocdv1.ApplyInstanceRequest{
		OrganizationId: c.organizationID,
		IdType:         idv1.Type_ID,
		Id:             instanceID,
		Clusters:       []*structpb.Struct{clusterPB},
	}, nil
}

func (c client) checkClusterReconciled(ctx context.Context, instanceID string, clusterName string) (*argocdv1.Cluster, error) {
	cluster, err := retry.DoWithData(
		func() (*argocdv1.Cluster, error) {
			c, err := c.GetCluster(ctx, instanceID, clusterName)
			if err != nil {
				return nil, err
			}

			reconStatus := c.GetReconciliationStatus()

			if reconStatus == nil || reconStatus.GetCode() != reconv1.StatusCode_STATUS_CODE_SUCCESSFUL && reconStatus.GetCode() != reconv1.StatusCode_STATUS_CODE_FAILED {
				return nil, reason.AsNotReconciled(errors.New("cluster has not yet been reconciled"))
			}

			return c, nil
		},
		retry.Context(ctx),
		retry.RetryIf(clusterNotFoundOrReconciledError),
		retry.Attempts(waitForReconciliationRetryAttempts),
		retry.Delay(time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
	)

	return cluster, err
}

func clusterNotFoundOrReconciledError(err error) bool {
	if reason.IsNotFound(err) || reason.IsNotReconciled(err) {
		return true
	}

	return false
}

func getManifestsFromResponse(respChan <-chan *httpbody.HttpBody, errChan <-chan error) (string, error) {
	var manifests string
	for {
		select {
		case dataChunk, ok := <-respChan:
			if !ok {
				respChan = nil
			} else {
				manifests += string(dataChunk.GetData())
			}

		case serverErr, ok := <-errChan:
			if !ok {
				errChan = nil
			} else if serverErr != nil {
				return "", fmt.Errorf("could not get cluster manifests from Akuity API, error: %w", serverErr)
			}
		}

		if respChan == nil || errChan == nil {
			break
		}
	}

	return manifests, nil
}
