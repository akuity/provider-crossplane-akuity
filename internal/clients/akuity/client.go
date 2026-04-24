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

	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
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
	// the Kargo / IpAllowList controllers where manifest-readiness is
	// surfaced via a controller-runtime requeue rather than a blocking
	// API wait.
	GetClusterManifestsOnce(ctx context.Context, instanceID, clusterID string) (string, error)
	DeleteCluster(ctx context.Context, instanceID string, name string) error
	GetInstance(ctx context.Context, name string) (*argocdv1.Instance, error)
	// GetInstanceByID fetches an Instance by its canonical ID. Used by
	// narrow-patch controllers that have the ID on their spec and want
	// to avoid a name-resolution round-trip.
	GetInstanceByID(ctx context.Context, id string) (*argocdv1.Instance, error)
	ExportInstance(ctx context.Context, name string) (*argocdv1.ExportInstanceResponse, error)
	// ExportInstanceByID is the ID-keyed variant of ExportInstance.
	// Used by narrow-owner controllers (Cluster) that carry the opaque
	// Akuity instance ID on their spec and need the canonical
	// round-trippable spec for drift comparison.
	ExportInstanceByID(ctx context.Context, id string) (*argocdv1.ExportInstanceResponse, error)
	// ApplyInstance narrow-merges the populated fields into the target
	// Instance; omitted fields are left untouched. Cluster/KargoAgent
	// child resources are applied by populating only the corresponding
	// Clusters/Agents slice on the request — build helpers live in the
	// owning controller's convert.go.
	ApplyInstance(ctx context.Context, request *argocdv1.ApplyInstanceRequest) error
	// PatchInstance merges the supplied structpb patch into the target
	// Instance's spec (keyed by ID). Narrow-patch controllers use this
	// to avoid the whole-spec Get+Apply dance that ApplyInstance requires.
	PatchInstance(ctx context.Context, id string, patch *structpb.Struct) error
	DeleteInstance(ctx context.Context, name string) error

	// Kargo-plane methods for the KargoInstance, KargoAgent, and
	// KargoDefaultShardAgent controllers. All routing is via the
	// KargoServiceGatewayClient configured alongside the Argo gateway
	// at construction time.
	GetKargoInstance(ctx context.Context, name string) (*kargov1.KargoInstance, error)
	// GetKargoInstanceByID fetches a KargoInstance by its canonical ID.
	// The Kargo GetKargoInstanceRequest only accepts a name; we emulate
	// the ID-keyed lookup by listing and filtering server-side (matches
	// how the Akuity Terraform provider resolves by ID).
	GetKargoInstanceByID(ctx context.Context, id string) (*kargov1.KargoInstance, error)
	ExportKargoInstance(ctx context.Context, name, workspaceID string) (*kargov1.ExportKargoInstanceResponse, error)
	// ApplyKargoInstance narrow-merges the populated fields into the
	// target KargoInstance; omitted fields (Kargo envelope, Projects,
	// Warehouses, Stages, RepoCredentials, …) are left untouched. Agent
	// child resources are applied by populating only the Agents slice
	// — see KargoAgent controller's convert.go for the build helper.
	ApplyKargoInstance(ctx context.Context, request *kargov1.ApplyKargoInstanceRequest) error
	// PatchKargoInstance merges the supplied structpb patch into the
	// target KargoInstance's spec (keyed by ID). Used by narrow-patch
	// controllers like KargoDefaultShardAgent.
	PatchKargoInstance(ctx context.Context, id string, patch *structpb.Struct) error
	DeleteKargoInstance(ctx context.Context, name string) error
	GetKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) (*kargov1.KargoAgent, error)
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
	return c.exportInstance(ctx, idv1.Type_NAME, name)
}

func (c client) ExportInstanceByID(ctx context.Context, id string) (*argocdv1.ExportInstanceResponse, error) {
	return c.exportInstance(ctx, idv1.Type_ID, id)
}

func (c client) exportInstance(ctx context.Context, idType idv1.Type, id string) (*argocdv1.ExportInstanceResponse, error) {
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.gatewayClient.ExportInstance(ctx, &argocdv1.ExportInstanceRequest{
		OrganizationId: c.organizationID,
		IdType:         idType,
		Id:             id,
	})

	if err != nil {
		if e, ok := status.FromError(err); ok {
			if e.Code() == codes.NotFound {
				return nil, reason.AsNotFound(fmt.Errorf("could not export instance %s from Akuity API, instance was not found", id))
			}
		}
		return nil, fmt.Errorf("could not export instance %s from Akuity API, error: %w", id, err)
	}

	if resp == nil {
		return nil, fmt.Errorf("could not export instance %s from Akuity API, response was empty", id)
	}

	return resp, nil
}

func (c client) ApplyInstance(ctx context.Context, request *argocdv1.ApplyInstanceRequest) error {
	// Auto-fill OrganizationId for callers that build the request
	// themselves (InstanceIpAllowList). Callers that go through
	// BuildApplyInstanceRequest / buildApplyClusterRequest already
	// set it; the guard is a no-op for them. Matches the shape used
	// by ApplyKargoInstance and the KargoInstanceAgent Create/Update
	// helpers below.
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

func (c client) ExportKargoInstance(ctx context.Context, name, workspaceID string) (*kargov1.ExportKargoInstanceResponse, error) {
	if err := c.kargoRequired("ExportKargoInstance"); err != nil {
		return nil, err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	// ExportKargoInstance looks the instance up by its canonical Id
	// (name or UUID); callers pass the name when they don't have an
	// ID handy, and the backend resolves either. The HTTP route is
	// workspace-scoped (no workspace-less fallback), so callers on
	// multi-workspace orgs must pass the correct workspaceID.
	resp, err := c.kargoGatewayClient.ExportKargoInstance(ctx, &kargov1.ExportKargoInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             name,
		WorkspaceId:    workspaceID,
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

// GetKargoInstanceAgent returns the agent identified by agentName (the
// user-facing Kargo agent name, e.g. the CR's spec.forProvider.name).
//
// The server's GetKargoInstanceAgent endpoint keys by opaque agent ID
// (models.KargoAgentWhere.ID.EQ(req.GetId())) with no name-lookup
// branch, so we resolve name→ID via ListKargoInstanceAgents first and
// then issue the Get by ID. The terraform-provider-akp resource does
// the same (terraform/akp/resource_akp_kargoagent.go:250-278).
//
// A NotFound from the resolve step surfaces as a NotFound from this
// function; callers treat that as "external resource doesn't exist
// yet" and flow into Create.
func (c client) GetKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) (*kargov1.KargoAgent, error) {
	if err := c.kargoRequired("GetKargoInstanceAgent"); err != nil {
		return nil, err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())

	agentID, rerr := c.resolveKargoAgentIDByName(ctx, kargoInstanceID, agentName)
	if rerr != nil {
		return nil, rerr
	}

	resp, err := c.kargoGatewayClient.GetKargoInstanceAgent(ctx, &kargov1.GetKargoInstanceAgentRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
		Id:             agentID,
	})
	if err != nil {
		if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
			return nil, reason.AsNotFound(fmt.Errorf("could not get kargo agent %s/%s (id=%s): %w", kargoInstanceID, agentName, agentID, err))
		}
		return nil, fmt.Errorf("could not get kargo agent %s/%s (id=%s): %w", kargoInstanceID, agentName, agentID, err)
	}
	if resp == nil || resp.GetAgent() == nil {
		return nil, fmt.Errorf("could not get kargo agent %s/%s (id=%s): empty response", kargoInstanceID, agentName, agentID)
	}
	return resp.GetAgent(), nil
}

// resolveKargoAgentIDByName scans ListKargoInstanceAgents for an agent
// whose Name matches. Returns a NotFound-wrapped error when no match
// is found so callers can rely on reason.IsNotFound.
func (c client) resolveKargoAgentIDByName(ctx context.Context, kargoInstanceID, agentName string) (string, error) {
	list, err := c.kargoGatewayClient.ListKargoInstanceAgents(ctx, &kargov1.ListKargoInstanceAgentsRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
	})
	if err != nil {
		if e, ok := status.FromError(err); ok && e.Code() == codes.NotFound {
			return "", reason.AsNotFound(fmt.Errorf("could not list kargo agents on instance %s: %w", kargoInstanceID, err))
		}
		return "", fmt.Errorf("could not list kargo agents on instance %s: %w", kargoInstanceID, err)
	}
	for _, a := range list.GetAgents() {
		if a == nil {
			continue
		}
		if a.GetName() == agentName {
			return a.GetId(), nil
		}
	}
	return "", reason.AsNotFound(fmt.Errorf("kargo agent %q not found on instance %s", agentName, kargoInstanceID))
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
