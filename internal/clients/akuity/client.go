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
	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
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
	// GetClusterManifestsOnce fetches install manifests for a cluster
	// without waiting for reconciliation. Callers are responsible for
	// gating invocation on the cluster's ReconciliationStatus. Used by
	// the v1alpha2 controllers where manifest-readiness is surfaced via
	// a controller-runtime requeue rather than a blocking API wait.
	GetClusterManifestsOnce(ctx context.Context, instanceID, clusterID string) (string, error)
	ApplyCluster(ctx context.Context, instanceID string, cluster akuitytypes.Cluster) error
	DeleteCluster(ctx context.Context, instanceID string, name string) error
	GetInstance(ctx context.Context, name string) (*argocdv1.Instance, error)
	// GetInstanceByID fetches an Instance by its canonical ID. Used by
	// narrow-patch controllers that have the ID on their spec and want
	// to avoid a name-resolution round-trip.
	GetInstanceByID(ctx context.Context, id string) (*argocdv1.Instance, error)
	ExportInstance(ctx context.Context, name string) (*argocdv1.ExportInstanceResponse, error)
	ApplyInstance(ctx context.Context, request *argocdv1.ApplyInstanceRequest) error
	// PatchInstance merges the supplied structpb patch into the target
	// Instance's spec (keyed by ID). Narrow-patch controllers use this
	// to avoid the whole-spec Get+Apply dance that ApplyInstance requires.
	PatchInstance(ctx context.Context, id string, patch *structpb.Struct) error
	DeleteInstance(ctx context.Context, name string) error
	BuildApplyInstanceRequest(instance crossplanetypes.Instance) (*argocdv1.ApplyInstanceRequest, error)

	// Kargo-plane methods (added for the v1alpha2 KargoInstance,
	// KargoAgent, and KargoDefaultShardAgent controllers). All routing
	// is via the KargoServiceGatewayClient configured alongside the
	// Argo gateway at construction time.
	GetKargoInstance(ctx context.Context, name string) (*kargov1.KargoInstance, error)
	// GetKargoInstanceByID fetches a KargoInstance by its canonical ID.
	// The Kargo GetKargoInstanceRequest only accepts a name; we emulate
	// the ID-keyed lookup by listing and filtering server-side (matches
	// how the Akuity Terraform provider resolves by ID).
	GetKargoInstanceByID(ctx context.Context, id string) (*kargov1.KargoInstance, error)
	ExportKargoInstance(ctx context.Context, name string) (*kargov1.ExportKargoInstanceResponse, error)
	ApplyKargoInstance(ctx context.Context, request *kargov1.ApplyKargoInstanceRequest) error
	// PatchKargoInstance merges the supplied structpb patch into the
	// target KargoInstance's spec (keyed by ID). Used by narrow-patch
	// controllers like KargoDefaultShardAgent.
	PatchKargoInstance(ctx context.Context, id string, patch *structpb.Struct) error
	DeleteKargoInstance(ctx context.Context, name string) error
	GetKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) (*kargov1.KargoAgent, error)
	CreateKargoInstanceAgent(ctx context.Context, request *kargov1.CreateKargoInstanceAgentRequest) (*kargov1.KargoAgent, error)
	UpdateKargoInstanceAgent(ctx context.Context, request *kargov1.UpdateKargoInstanceAgentRequest) (*kargov1.KargoAgent, error)
	DeleteKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) error
	// GetKargoInstanceAgentManifestsOnce fetches install manifests for
	// a Kargo agent without waiting for reconciliation.
	GetKargoInstanceAgentManifestsOnce(ctx context.Context, kargoInstanceID, agentID string) (string, error)
}

type client struct {
	organizationID     string
	credentials        accesscontrol.ClientCredential
	gatewayClient      argocdv1.ArgoCDServiceGatewayClient
	kargoGatewayClient kargov1.KargoServiceGatewayClient
}

// NewClient constructs an Akuity client backed by both the ArgoCD and
// Kargo gateway clients. kargoGatewayClient may be nil when the
// caller only intends to use Argo-plane methods (e.g. in legacy
// tests); Kargo methods will then return a descriptive error.
func NewClient(organizationID string, apiKeyID string, apiKeySecret string, gatewayClient argocdv1.ArgoCDServiceGatewayClient, kargoGatewayClient kargov1.KargoServiceGatewayClient) (Client, error) {
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
		organizationID:     organizationID,
		credentials:        accesscontrol.NewAPIKeyCredential(apiKeyID, apiKeySecret),
		gatewayClient:      gatewayClient,
		kargoGatewayClient: kargoGatewayClient,
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
		// The Akuity API does not distinguish NotFound from PermissionDenied
		// when reading organisation-scoped clusters. See internal/reason/doc.go.
		if e, ok := status.FromError(err); ok {
			if e.Code() == codes.NotFound || e.Code() == codes.PermissionDenied {
				return nil, reason.AsNotFound(fmt.Errorf("could not get cluster %s from Akuity API: %w", name, err))
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

func (c client) GetClusterManifestsOnce(ctx context.Context, instanceID, clusterID string) (string, error) {
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	respChan, errChan, err := c.gatewayClient.GetInstanceClusterManifests(ctx, &argocdv1.GetInstanceClusterManifestsRequest{
		OrganizationId: c.organizationID,
		InstanceId:     instanceID,
		Id:             clusterID,
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

func (c client) GetInstanceByID(ctx context.Context, id string) (*argocdv1.Instance, error) {
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.gatewayClient.GetInstance(ctx, &argocdv1.GetInstanceRequest{
		OrganizationId: c.organizationID,
		IdType:         idv1.Type_ID,
		Id:             id,
	})
	if err != nil {
		if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
			return nil, reason.AsNotFound(fmt.Errorf("could not get instance %s from Akuity API, instance was not found", id))
		}
		return nil, fmt.Errorf("could not get instance %s from Akuity API, error: %w", id, err)
	}
	if resp == nil || resp.GetInstance() == nil {
		return nil, fmt.Errorf("could not get instance %s from Akuity API, response was empty", id)
	}
	return resp.GetInstance(), nil
}

func (c client) PatchInstance(ctx context.Context, id string, patch *structpb.Struct) error {
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	_, err := c.gatewayClient.PatchInstance(ctx, &argocdv1.PatchInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             id,
		Patch:          patch,
	})
	if err != nil {
		return fmt.Errorf("could not patch instance %s using Akuity API, error: %w", id, err)
	}
	return nil
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
	// Auto-fill OrganizationId for callers that build the request
	// themselves (v1alpha2 Instance / InstanceIpAllowList controllers).
	// Legacy callers that go through BuildApplyInstanceRequest /
	// buildApplyClusterRequest already set it; the guard is a no-op
	// for them. Matches the shape used by ApplyKargoInstance and the
	// KargoInstanceAgent Create/Update helpers below.
	if request.GetOrganizationId() == "" {
		request.OrganizationId = c.organizationID
	}
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

// ----------------------------------------------------------------------
// Kargo-plane methods. All take an instance name (or ID) as the Akuity
// API's canonical lookup key; controllers that drive them are expected
// to carry IDs on the managed resource's AtProvider block to avoid
// repeated name→ID resolutions.
// ----------------------------------------------------------------------

func (c client) kargoRequired(op string) error {
	if c.kargoGatewayClient == nil {
		return fmt.Errorf("%s: kargo gateway client not configured on this Akuity client", op)
	}
	return nil
}

func (c client) GetKargoInstance(ctx context.Context, name string) (*kargov1.KargoInstance, error) {
	if err := c.kargoRequired("GetKargoInstance"); err != nil {
		return nil, err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.kargoGatewayClient.GetKargoInstance(ctx, &kargov1.GetKargoInstanceRequest{
		OrganizationId: c.organizationID,
		Name:           name,
	})
	if err != nil {
		if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
			return nil, reason.AsNotFound(fmt.Errorf("could not get kargo instance %s: %w", name, err))
		}
		return nil, fmt.Errorf("could not get kargo instance %s: %w", name, err)
	}
	if resp == nil || resp.GetInstance() == nil {
		return nil, fmt.Errorf("could not get kargo instance %s: empty response", name)
	}
	return resp.GetInstance(), nil
}

func (c client) GetKargoInstanceByID(ctx context.Context, id string) (*kargov1.KargoInstance, error) {
	if err := c.kargoRequired("GetKargoInstanceByID"); err != nil {
		return nil, err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	// GetKargoInstanceRequest only accepts a name. Emulate ID-keyed
	// lookup via ListKargoInstances + client-side filter (same approach
	// as akuityio/akuity-platform/terraform/akp).
	list, err := c.kargoGatewayClient.ListKargoInstances(ctx, &kargov1.ListKargoInstancesRequest{
		OrganizationId: c.organizationID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not list kargo instances while resolving %s: %w", id, err)
	}
	for _, inst := range list.GetInstances() {
		if inst.GetId() == id {
			return inst, nil
		}
	}
	return nil, reason.AsNotFound(fmt.Errorf("kargo instance %s not found", id))
}

func (c client) PatchKargoInstance(ctx context.Context, id string, patch *structpb.Struct) error {
	if err := c.kargoRequired("PatchKargoInstance"); err != nil {
		return err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	_, err := c.kargoGatewayClient.PatchKargoInstance(ctx, &kargov1.PatchKargoInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             id,
		Patch:          patch,
	})
	if err != nil {
		return fmt.Errorf("could not patch kargo instance %s: %w", id, err)
	}
	return nil
}

func (c client) ExportKargoInstance(ctx context.Context, name string) (*kargov1.ExportKargoInstanceResponse, error) {
	if err := c.kargoRequired("ExportKargoInstance"); err != nil {
		return nil, err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	// ExportKargoInstance looks the instance up by its canonical Id
	// (name or UUID); callers pass the name when they don't have an
	// ID handy, and the backend resolves either.
	resp, err := c.kargoGatewayClient.ExportKargoInstance(ctx, &kargov1.ExportKargoInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             name,
	})
	if err != nil {
		if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
			return nil, reason.AsNotFound(fmt.Errorf("could not export kargo instance %s: %w", name, err))
		}
		return nil, fmt.Errorf("could not export kargo instance %s: %w", name, err)
	}
	return resp, nil
}

func (c client) ApplyKargoInstance(ctx context.Context, request *kargov1.ApplyKargoInstanceRequest) error {
	if err := c.kargoRequired("ApplyKargoInstance"); err != nil {
		return err
	}
	if request.GetOrganizationId() == "" {
		request.OrganizationId = c.organizationID
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	_, err := c.kargoGatewayClient.ApplyKargoInstance(ctx, request)
	return err
}

func (c client) DeleteKargoInstance(ctx context.Context, name string) error {
	if err := c.kargoRequired("DeleteKargoInstance"); err != nil {
		return err
	}
	inst, err := c.GetKargoInstance(ctx, name)
	if err != nil {
		return err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	_, err = c.kargoGatewayClient.DeleteInstance(ctx, &kargov1.DeleteInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             inst.GetId(),
	})
	if err != nil {
		return fmt.Errorf("could not delete kargo instance %s: %w", name, err)
	}
	return nil
}

func (c client) GetKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) (*kargov1.KargoAgent, error) {
	if err := c.kargoRequired("GetKargoInstanceAgent"); err != nil {
		return nil, err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.kargoGatewayClient.GetKargoInstanceAgent(ctx, &kargov1.GetKargoInstanceAgentRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
		Id:             agentName,
	})
	if err != nil {
		if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
			return nil, reason.AsNotFound(fmt.Errorf("could not get kargo agent %s/%s: %w", kargoInstanceID, agentName, err))
		}
		return nil, fmt.Errorf("could not get kargo agent %s/%s: %w", kargoInstanceID, agentName, err)
	}
	if resp == nil || resp.GetAgent() == nil {
		return nil, fmt.Errorf("could not get kargo agent %s/%s: empty response", kargoInstanceID, agentName)
	}
	return resp.GetAgent(), nil
}

func (c client) CreateKargoInstanceAgent(ctx context.Context, request *kargov1.CreateKargoInstanceAgentRequest) (*kargov1.KargoAgent, error) {
	if err := c.kargoRequired("CreateKargoInstanceAgent"); err != nil {
		return nil, err
	}
	if request.GetOrganizationId() == "" {
		request.OrganizationId = c.organizationID
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.kargoGatewayClient.CreateKargoInstanceAgent(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("could not create kargo agent: %w", err)
	}
	return resp.GetAgent(), nil
}

func (c client) UpdateKargoInstanceAgent(ctx context.Context, request *kargov1.UpdateKargoInstanceAgentRequest) (*kargov1.KargoAgent, error) {
	if err := c.kargoRequired("UpdateKargoInstanceAgent"); err != nil {
		return nil, err
	}
	if request.GetOrganizationId() == "" {
		request.OrganizationId = c.organizationID
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.kargoGatewayClient.UpdateKargoInstanceAgent(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("could not update kargo agent: %w", err)
	}
	return resp.GetAgent(), nil
}

func (c client) GetKargoInstanceAgentManifestsOnce(ctx context.Context, kargoInstanceID, agentID string) (string, error) {
	if err := c.kargoRequired("GetKargoInstanceAgentManifestsOnce"); err != nil {
		return "", err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	respChan, errChan, err := c.kargoGatewayClient.GetKargoInstanceAgentManifests(ctx, &kargov1.GetKargoInstanceAgentManifestsRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
		Id:             agentID,
	})
	if err != nil {
		return "", fmt.Errorf("could not get kargo agent manifests: %w", err)
	}
	return getManifestsFromResponse(respChan, errChan)
}

func (c client) DeleteKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) error {
	if err := c.kargoRequired("DeleteKargoInstanceAgent"); err != nil {
		return err
	}
	agent, err := c.GetKargoInstanceAgent(ctx, kargoInstanceID, agentName)
	if err != nil {
		return err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	_, err = c.kargoGatewayClient.DeleteInstanceAgent(ctx, &kargov1.DeleteInstanceAgentRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
		Id:             agent.GetId(),
	})
	if err != nil {
		return fmt.Errorf("could not delete kargo agent %s/%s: %w", kargoInstanceID, agentName, err)
	}
	return nil
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
