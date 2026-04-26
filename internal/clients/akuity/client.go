package akuity

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/avast/retry-go/v4"

	"github.com/akuity/api-client-go/pkg/api/gateway/accesscontrol"
	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	orgcv1 "github.com/akuity/api-client-go/pkg/api/gen/organization/v1"
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
	// SetClusterMaintenanceMode targets the dedicated set-maintenance-mode
	// gateway route. ApplyInstance does not propagate
	// data.maintenanceMode or data.maintenanceModeExpiry, so those
	// fields flow through this separate RPC. Pass expiry=nil when
	// maintenance mode has no time bound on the platform.
	SetClusterMaintenanceMode(ctx context.Context, instanceID, clusterName string, mode bool, expiry *time.Time) error
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
	// Clusters/Agents slice on the request; build helpers live in the
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
	// the ID-keyed lookup by listing and filtering server-side.
	GetKargoInstanceByID(ctx context.Context, id string) (*kargov1.KargoInstance, error)
	ExportKargoInstance(ctx context.Context, name, workspaceID string) (*kargov1.ExportKargoInstanceResponse, error)
	// ApplyKargoInstance narrow-merges the populated fields into the
	// target KargoInstance; omitted fields (Kargo envelope, Projects,
	// Warehouses, Stages, RepoCredentials, etc.) are left untouched. Agent
	// child resources are applied by populating only the Agents slice
	// in the KargoAgent controller's convert.go build helper.
	ApplyKargoInstance(ctx context.Context, request *kargov1.ApplyKargoInstanceRequest) error
	// PatchKargoInstance merges the supplied structpb patch into the
	// target KargoInstance's spec (keyed by ID). Used by narrow-patch
	// controllers like KargoDefaultShardAgent.
	PatchKargoInstance(ctx context.Context, id string, patch *structpb.Struct) error
	DeleteKargoInstance(ctx context.Context, name string) error
	GetKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) (*kargov1.KargoAgent, error)
	DeleteKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) error
	// GetKargoInstanceAgentManifests fetches install manifests for a
	// Kargo agent AFTER blocking until the agent reaches
	// SUCCESSFUL/FAILED reconciliation. Mirrors GetClusterManifests +
	// checkClusterReconciled for the argo plane. Used by the KargoAgent
	// controller's Create path where manifest install has to happen
	// atomically (a transient Unavailable on the manifests stream would
	// otherwise leave the platform row stamped without ever installing
	// the agent on the managed cluster).
	GetKargoInstanceAgentManifests(ctx context.Context, kargoInstanceID, agentName string) (string, error)
	// GetKargoInstanceAgentManifestsOnce fetches install manifests for
	// a Kargo agent without waiting for reconciliation.
	GetKargoInstanceAgentManifestsOnce(ctx context.Context, kargoInstanceID, agentID string) (string, error)

	// ResolveWorkspace resolves an Akuity workspace by ID or name and
	// returns it. When name is empty the organization's default workspace is
	// returned. Used by KargoInstance to avoid hot-looping ApplyKargoInstance
	// against a workspace-scoped HTTP route with an empty workspace_id
	// (the route templates the id straight into the path so empty produces
	// a 404).
	ResolveWorkspace(ctx context.Context, name string) (*orgcv1.Workspace, error)
}

type client struct {
	organizationID     string
	credentials        accesscontrol.ClientCredential
	gatewayClient      argocdv1.ArgoCDServiceGatewayClient
	kargoGatewayClient kargov1.KargoServiceGatewayClient
	orgGatewayClient   orgcv1.OrganizationServiceGatewayClient
	workspaceCache     *workspaceIDCache
}

type workspaceIDCache struct {
	mu              sync.RWMutex
	refs            map[string]string
	argoByInstance  map[string]string
	kargoByInstance map[string]string
}

func newWorkspaceIDCache() *workspaceIDCache {
	return &workspaceIDCache{
		refs:            map[string]string{},
		argoByInstance:  map[string]string{},
		kargoByInstance: map[string]string{},
	}
}

func workspaceRefCacheKey(ref string) string {
	if ref == "" {
		return "<default>"
	}
	return ref
}

func (c *workspaceIDCache) getRef(ref string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.refs[workspaceRefCacheKey(ref)]
	return v, ok
}

func (c *workspaceIDCache) setRef(ref, workspaceID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refs[workspaceRefCacheKey(ref)] = workspaceID
}

func (c *workspaceIDCache) getArgoInstance(instanceID string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.argoByInstance[instanceID]
	return v, ok
}

func (c *workspaceIDCache) setArgoInstance(instanceID, workspaceID string) {
	if c == nil || instanceID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.argoByInstance[instanceID] = workspaceID
}

func (c *workspaceIDCache) getKargoInstance(instanceID string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.kargoByInstance[instanceID]
	return v, ok
}

func (c *workspaceIDCache) setKargoInstance(instanceID, workspaceID string) {
	if c == nil || instanceID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.kargoByInstance[instanceID] = workspaceID
}

// NewClient constructs an Akuity client backed by the ArgoCD, Kargo,
// and Organization gateway clients. kargoGatewayClient and
// orgGatewayClient may be nil when the caller only intends to use a
// subset of the API surface (e.g. in legacy tests that exercise only
// the Argo plane); the corresponding methods will then return a
// descriptive error rather than panicking on a nil dispatch.
func NewClient(organizationID string, apiKeyID string, apiKeySecret string, gatewayClient argocdv1.ArgoCDServiceGatewayClient, kargoGatewayClient kargov1.KargoServiceGatewayClient, orgGatewayClient orgcv1.OrganizationServiceGatewayClient) (Client, error) {
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
		orgGatewayClient:   orgGatewayClient,
		workspaceCache:     newWorkspaceIDCache(),
	}

	return c, nil
}

func (c client) resolveWorkspaceID(ctx context.Context, ref string) (string, error) {
	if c.orgGatewayClient == nil {
		return ref, nil
	}
	if workspaceID, ok := c.workspaceCache.getRef(ref); ok {
		return workspaceID, nil
	}
	w, err := c.ResolveWorkspace(ctx, ref)
	if err != nil {
		if ref != "" && reason.IsNotFound(err) {
			return "", reason.AsTerminal(fmt.Errorf("workspace %q: %w", ref, err))
		}
		return "", err
	}
	workspaceID := w.GetId()
	c.workspaceCache.setRef(ref, workspaceID)
	return workspaceID, nil
}

func (c client) argoWorkspaceIDForInstance(ctx context.Context, instanceID string) (string, error) {
	if workspaceID, ok := c.workspaceCache.getArgoInstance(instanceID); ok {
		return workspaceID, nil
	}
	inst, err := c.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return "", err
	}
	if ws := inst.GetWorkspaceId(); ws != "" {
		c.workspaceCache.setArgoInstance(instanceID, ws)
		return ws, nil
	}
	workspaceID, err := c.resolveWorkspaceID(ctx, "")
	if err != nil {
		return "", err
	}
	c.workspaceCache.setArgoInstance(instanceID, workspaceID)
	return workspaceID, nil
}

func (c client) argoWorkspaceIDForApply(ctx context.Context, req *argocdv1.ApplyInstanceRequest) (string, error) {
	if ws := req.GetWorkspaceId(); ws != "" {
		return c.resolveWorkspaceID(ctx, ws)
	}
	if req.GetIdType() == idv1.Type_ID {
		return c.argoWorkspaceIDForInstance(ctx, req.GetId())
	}
	if c.orgGatewayClient == nil {
		return "", nil
	}
	inst, err := c.GetInstance(ctx, req.GetId())
	if err == nil {
		if ws := inst.GetWorkspaceId(); ws != "" {
			c.workspaceCache.setArgoInstance(inst.GetId(), ws)
			return ws, nil
		}
		return c.resolveWorkspaceID(ctx, "")
	}
	if !reason.IsNotFound(err) {
		return "", err
	}
	return c.resolveWorkspaceID(ctx, "")
}

func (c client) kargoWorkspaceIDForInstance(ctx context.Context, instanceID string) (string, error) {
	if workspaceID, ok := c.workspaceCache.getKargoInstance(instanceID); ok {
		return workspaceID, nil
	}
	inst, err := c.GetKargoInstanceByID(ctx, instanceID)
	if err != nil {
		return "", err
	}
	if ws := inst.GetWorkspaceId(); ws != "" {
		c.workspaceCache.setKargoInstance(instanceID, ws)
		return ws, nil
	}
	workspaceID, err := c.resolveWorkspaceID(ctx, "")
	if err != nil {
		return "", err
	}
	c.workspaceCache.setKargoInstance(instanceID, workspaceID)
	return workspaceID, nil
}

func (c client) kargoWorkspaceIDForRef(ctx context.Context, ref string) (string, error) {
	if workspaceID, ok := c.workspaceCache.getKargoInstance(ref); ok {
		return workspaceID, nil
	}
	inst, err := c.GetKargoInstanceByID(ctx, ref)
	if reason.IsNotFound(err) {
		inst, err = c.GetKargoInstance(ctx, ref)
	}
	if err != nil {
		return "", err
	}
	if ws := inst.GetWorkspaceId(); ws != "" {
		c.workspaceCache.setKargoInstance(ref, ws)
		c.workspaceCache.setKargoInstance(inst.GetId(), ws)
		return ws, nil
	}
	workspaceID, err := c.resolveWorkspaceID(ctx, "")
	if err != nil {
		return "", err
	}
	c.workspaceCache.setKargoInstance(ref, workspaceID)
	c.workspaceCache.setKargoInstance(inst.GetId(), workspaceID)
	return workspaceID, nil
}

func (c client) kargoWorkspaceIDForApply(ctx context.Context, req *kargov1.ApplyKargoInstanceRequest) (string, error) {
	if ws := req.GetWorkspaceId(); ws != "" {
		return c.resolveWorkspaceID(ctx, ws)
	}
	if req.GetIdType() == idv1.Type_ID {
		return c.kargoWorkspaceIDForInstance(ctx, req.GetId())
	}
	if c.orgGatewayClient == nil {
		return "", nil
	}
	inst, err := c.GetKargoInstance(ctx, req.GetId())
	if err == nil {
		if ws := inst.GetWorkspaceId(); ws != "" {
			c.workspaceCache.setKargoInstance(inst.GetId(), ws)
			return ws, nil
		}
		return c.resolveWorkspaceID(ctx, "")
	}
	if !reason.IsNotFound(err) {
		return "", err
	}
	return c.resolveWorkspaceID(ctx, "")
}

func (c client) GetCluster(ctx context.Context, instanceID string, name string) (*argocdv1.Cluster, error) {
	workspaceID, err := c.argoWorkspaceIDForInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.gatewayClient.GetInstanceCluster(ctx, &argocdv1.GetInstanceClusterRequest{
		OrganizationId: c.organizationID,
		InstanceId:     instanceID,
		WorkspaceId:    workspaceID,
		IdType:         idv1.Type_NAME,
		Id:             name,
	})

	if err != nil {
		// The Akuity API does not distinguish NotFound from PermissionDenied
		// when reading organization-scoped clusters. See internal/reason/doc.go.
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
	workspaceID, err := c.argoWorkspaceIDForInstance(ctx, instanceID)
	if err != nil {
		return "", err
	}

	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	respChan, errChan, err := c.gatewayClient.GetInstanceClusterManifests(ctx, &argocdv1.GetInstanceClusterManifestsRequest{
		OrganizationId: c.organizationID,
		InstanceId:     instanceID,
		WorkspaceId:    workspaceID,
		Id:             cluster.GetId(),
	})

	if err != nil {
		return "", fmt.Errorf("could not get cluster manifests from Akuity API, error: %w", err)
	}

	return getManifestsFromResponse(respChan, errChan)
}

func (c client) GetClusterManifestsOnce(ctx context.Context, instanceID, clusterID string) (string, error) {
	workspaceID, err := c.argoWorkspaceIDForInstance(ctx, instanceID)
	if err != nil {
		return "", err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	respChan, errChan, err := c.gatewayClient.GetInstanceClusterManifests(ctx, &argocdv1.GetInstanceClusterManifestsRequest{
		OrganizationId: c.organizationID,
		InstanceId:     instanceID,
		WorkspaceId:    workspaceID,
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
	workspaceID, err := c.argoWorkspaceIDForInstance(ctx, instanceID)
	if err != nil {
		return err
	}

	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	incAPIWrite("DeleteCluster", instanceID+"/"+cluster.GetId())
	_, err = c.gatewayClient.DeleteInstanceCluster(ctx, &argocdv1.DeleteInstanceClusterRequest{
		OrganizationId: c.organizationID,
		InstanceId:     instanceID,
		WorkspaceId:    workspaceID,
		Id:             cluster.GetId(),
	})

	if err != nil {
		return fmt.Errorf("could not delete cluster %s using Akuity API, error: %w", name, err)
	}

	return nil
}

// SetClusterMaintenanceMode implements Client.SetClusterMaintenanceMode.
//
// The Akuity gateway exposes maintenance state through a dedicated
// /clusters/set-maintenance-mode route rather than the standard
// ApplyInstance Cluster sub-payload. ApplyInstance silently drops
// data.maintenanceMode / data.maintenanceModeExpiry, so without this
// separate call the user-set maintenance state never reaches the
// platform and the drift comparator hot-loops Apply on every poll.
func (c client) SetClusterMaintenanceMode(ctx context.Context, instanceID, clusterName string, mode bool, expiry *time.Time) error {
	workspaceID, err := c.argoWorkspaceIDForInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	req := &argocdv1.SetClusterMaintenanceModeRequest{
		OrganizationId:  c.organizationID,
		InstanceId:      instanceID,
		WorkspaceId:     workspaceID,
		ClusterNames:    []string{clusterName},
		MaintenanceMode: mode,
	}
	if expiry != nil {
		req.Expiry = timestamppb.New(*expiry)
	}
	incAPIWrite("SetClusterMaintenanceMode", instanceID+"/"+clusterName)
	if _, err := c.gatewayClient.SetClusterMaintenanceMode(ctx, req); err != nil {
		return fmt.Errorf("could not set maintenance mode for cluster %s/%s: %w", instanceID, clusterName, err)
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
	workspaceID, err := c.argoWorkspaceIDForInstance(ctx, id)
	if err != nil {
		return err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	incAPIWrite("PatchInstance", id)
	_, err = c.gatewayClient.PatchInstance(ctx, &argocdv1.PatchInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             id,
		WorkspaceId:    workspaceID,
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
	workspaceID, err := c.argoWorkspaceIDForApply(ctx, request)
	if err != nil {
		return err
	}
	request.WorkspaceId = workspaceID
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	incAPIWrite("ApplyInstance", request.GetId())
	_, err = c.gatewayClient.ApplyInstance(ctx, request)

	return err
}

func (c client) DeleteInstance(ctx context.Context, name string) error {
	instance, err := c.GetInstance(ctx, name)
	if err != nil {
		return fmt.Errorf("could not get instance %s to delete, err: %w", name, err)
	}
	workspaceID := instance.GetWorkspaceId()
	if workspaceID == "" {
		workspaceID, err = c.resolveWorkspaceID(ctx, "")
		if err != nil {
			return err
		}
	}

	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	incAPIWrite("DeleteInstance", instance.GetId())
	_, err = c.gatewayClient.DeleteInstance(ctx, &argocdv1.DeleteInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             instance.GetId(),
		WorkspaceId:    workspaceID,
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
// repeated name-to-ID resolutions.
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
	// lookup via ListKargoInstances plus a client-side filter.
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
	workspaceID, err := c.kargoWorkspaceIDForInstance(ctx, id)
	if err != nil {
		return err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	incAPIWrite("PatchKargoInstance", id)
	_, err = c.kargoGatewayClient.PatchKargoInstance(ctx, &kargov1.PatchKargoInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             id,
		WorkspaceId:    workspaceID,
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
	var (
		resolvedWorkspaceID string
		err                 error
	)
	if workspaceID != "" {
		resolvedWorkspaceID, err = c.resolveWorkspaceID(ctx, workspaceID)
	} else {
		resolvedWorkspaceID, err = c.kargoWorkspaceIDForRef(ctx, name)
		if err != nil {
			if !reason.IsNotFound(err) {
				return nil, err
			}
			resolvedWorkspaceID, err = c.resolveWorkspaceID(ctx, "")
		}
	}
	if err != nil {
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
		WorkspaceId:    resolvedWorkspaceID,
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
	workspaceID, err := c.kargoWorkspaceIDForApply(ctx, request)
	if err != nil {
		return err
	}
	request.WorkspaceId = workspaceID
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	incAPIWrite("ApplyKargoInstance", request.GetId())
	_, err = c.kargoGatewayClient.ApplyKargoInstance(ctx, request)
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
	workspaceID := inst.GetWorkspaceId()
	if workspaceID == "" {
		workspaceID, err = c.resolveWorkspaceID(ctx, "")
		if err != nil {
			return err
		}
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	incAPIWrite("DeleteKargoInstance", inst.GetId())
	_, err = c.kargoGatewayClient.DeleteInstance(ctx, &kargov1.DeleteInstanceRequest{
		OrganizationId: c.organizationID,
		Id:             inst.GetId(),
		WorkspaceId:    workspaceID,
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
// with no name-lookup branch, so resolve name to ID via
// ListKargoInstanceAgents first and then issue the Get by ID.
//
// A NotFound from the resolve step surfaces as a NotFound from this
// function; callers treat that as "external resource doesn't exist
// yet" and flow into Create.
func (c client) GetKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) (*kargov1.KargoAgent, error) {
	if err := c.kargoRequired("GetKargoInstanceAgent"); err != nil {
		return nil, err
	}
	workspaceID, err := c.kargoWorkspaceIDForInstance(ctx, kargoInstanceID)
	if err != nil {
		return nil, err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())

	agentID, rerr := c.resolveKargoAgentIDByName(ctx, kargoInstanceID, agentName, workspaceID)
	if rerr != nil {
		return nil, rerr
	}

	resp, err := c.kargoGatewayClient.GetKargoInstanceAgent(ctx, &kargov1.GetKargoInstanceAgentRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
		WorkspaceId:    workspaceID,
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
func (c client) resolveKargoAgentIDByName(ctx context.Context, kargoInstanceID, agentName, workspaceID string) (string, error) {
	list, err := c.kargoGatewayClient.ListKargoInstanceAgents(ctx, &kargov1.ListKargoInstanceAgentsRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
		WorkspaceId:    workspaceID,
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
	workspaceID, err := c.kargoWorkspaceIDForInstance(ctx, kargoInstanceID)
	if err != nil {
		return "", err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	respChan, errChan, err := c.kargoGatewayClient.GetKargoInstanceAgentManifests(ctx, &kargov1.GetKargoInstanceAgentManifestsRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
		WorkspaceId:    workspaceID,
		Id:             agentID,
	})
	if err != nil {
		return "", fmt.Errorf("could not get kargo agent manifests: %w", err)
	}
	return getManifestsFromResponse(respChan, errChan)
}

// GetKargoInstanceAgentManifests blocks until the agent reaches a
// terminal reconciliation state (SUCCESSFUL/FAILED), then streams the
// install manifests. This mirrors GetClusterManifests: the first Create
// after ApplyKargoInstance can race the gateway's manifest renderer.
// This helper waits before opening the manifest stream.
func (c client) GetKargoInstanceAgentManifests(ctx context.Context, kargoInstanceID, agentName string) (string, error) {
	if err := c.kargoRequired("GetKargoInstanceAgentManifests"); err != nil {
		return "", err
	}
	agent, err := c.checkKargoAgentReconciled(ctx, kargoInstanceID, agentName)
	if err != nil {
		return "", err
	}
	workspaceID, err := c.kargoWorkspaceIDForInstance(ctx, kargoInstanceID)
	if err != nil {
		return "", err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	respChan, errChan, err := c.kargoGatewayClient.GetKargoInstanceAgentManifests(ctx, &kargov1.GetKargoInstanceAgentManifestsRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
		WorkspaceId:    workspaceID,
		Id:             agent.GetId(),
	})
	if err != nil {
		return "", fmt.Errorf("could not get kargo agent manifests: %w", err)
	}
	return getManifestsFromResponse(respChan, errChan)
}

func (c client) checkKargoAgentReconciled(ctx context.Context, kargoInstanceID, agentName string) (*kargov1.KargoAgent, error) {
	agent, err := retry.DoWithData(
		func() (*kargov1.KargoAgent, error) {
			a, err := c.GetKargoInstanceAgent(ctx, kargoInstanceID, agentName)
			if err != nil {
				return nil, err
			}
			reconStatus := a.GetReconciliationStatus()
			if reconStatus == nil || reconStatus.GetCode() != reconv1.StatusCode_STATUS_CODE_SUCCESSFUL && reconStatus.GetCode() != reconv1.StatusCode_STATUS_CODE_FAILED {
				return nil, reason.AsNotReconciled(errors.New("kargo agent has not yet been reconciled"))
			}
			return a, nil
		},
		retry.Context(ctx),
		retry.RetryIf(kargoAgentNotFoundOrReconciledError),
		retry.Attempts(waitForReconciliationRetryAttempts),
		retry.Delay(time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
	)
	return agent, err
}

func kargoAgentNotFoundOrReconciledError(err error) bool {
	if reason.IsNotFound(err) || reason.IsNotReconciled(err) {
		return true
	}
	if e, ok := status.FromError(err); ok && e.Code() == codes.Unavailable {
		return true
	}
	return false
}

func (c client) DeleteKargoInstanceAgent(ctx context.Context, kargoInstanceID, agentName string) error {
	if err := c.kargoRequired("DeleteKargoInstanceAgent"); err != nil {
		return err
	}
	agent, err := c.GetKargoInstanceAgent(ctx, kargoInstanceID, agentName)
	if err != nil {
		return err
	}
	workspaceID, err := c.kargoWorkspaceIDForInstance(ctx, kargoInstanceID)
	if err != nil {
		return err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	incAPIWrite("DeleteKargoInstanceAgent", kargoInstanceID+"/"+agent.GetId())
	_, err = c.kargoGatewayClient.DeleteInstanceAgent(ctx, &kargov1.DeleteInstanceAgentRequest{
		OrganizationId: c.organizationID,
		InstanceId:     kargoInstanceID,
		WorkspaceId:    workspaceID,
		Id:             agent.GetId(),
	})
	if err != nil {
		return fmt.Errorf("could not delete kargo agent %s/%s: %w", kargoInstanceID, agentName, err)
	}
	return nil
}

func (c client) orgRequired(op string) error {
	if c.orgGatewayClient == nil {
		return fmt.Errorf("%s: organization gateway client not configured on this Akuity client", op)
	}
	return nil
}

// ResolveWorkspace implements Client.ResolveWorkspace.
//
// When name is empty the function selects the workspace flagged
// IsDefault by the Akuity portal, which is what the platform expects
// for single-workspace organizations.
// When name is non-empty the function first matches workspace ID, then
// workspace name. ID wins because gateway routes require the canonical ID.
// The matching workspace is returned, with
// codes.NotFound semantics surfaced via reason.NotFound when no match
// exists so callers can distinguish "workspace not found" from
// "transient gateway error".
func (c client) ResolveWorkspace(ctx context.Context, name string) (*orgcv1.Workspace, error) { //nolint:gocyclo // Branching is naturally one-per-state for the dual ID/name resolution path; splitting into helpers obscures the intended precedence.
	if err := c.orgRequired("ResolveWorkspace"); err != nil {
		return nil, err
	}
	ctx = httpctx.SetAuthorizationHeader(ctx, c.credentials.Scheme(), c.credentials.Credential())
	resp, err := c.orgGatewayClient.ListWorkspaces(ctx, &orgcv1.ListWorkspacesRequest{
		OrganizationId: c.organizationID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not list workspaces: %w", err)
	}
	var nameMatch *orgcv1.Workspace
	for _, w := range resp.GetWorkspaces() {
		if name == "" {
			if w.GetIsDefault() {
				return w, nil
			}
			continue
		}
		if w.GetId() == name {
			return w, nil
		}
		if nameMatch == nil && w.GetName() == name {
			nameMatch = w
		}
	}
	if name == "" {
		return nil, reason.AsNotFound(fmt.Errorf("default workspace not found in organization %s", c.organizationID))
	}
	if nameMatch != nil {
		return nameMatch, nil
	}
	return nil, reason.AsNotFound(fmt.Errorf("workspace %q not found in organization %s", name, c.organizationID))
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
