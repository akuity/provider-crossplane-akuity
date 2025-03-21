package fixtures

import (
	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	reconciliation "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	generated "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/protobuf"
)

var (
	ClusterName         = "test-cluster"
	AutoUpgradeDisabled = true
	AppReplication      = true
	RedisTunneling      = true
	KustomizationYAML   = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
patches:
- patch: |-
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: argocd-repo-server
    spec:
      template:
        spec:
          containers:
          - name: argocd-repo-server
            resources:
              limits:
                memory: 2Gi
              requests:
                cpu: 750m
                memory: 1Gi
    target:
      kind: Deployment
      name: argocd-repo-server
`
	KustomizationJSON, _ = yaml.YAMLToJSON([]byte(KustomizationYAML))
	Kustomization        = runtime.RawExtension{Raw: KustomizationJSON}
	KustomizationPB, _   = protobuf.MarshalObjectToProtobufStruct(Kustomization)
	Labels               = map[string]string{"key1": "value1", "key2": "value2"}
	Annotations          = map[string]string{"annotation1": "value1", "annotation2": "value2"}

	CrossplaneManagedCluster = v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			ForProvider: CrossplaneCluster,
		},
	}

	CrossplaneCluster = v1alpha1.ClusterParameters{
		InstanceID: InstanceID,
		InstanceRef: v1alpha1.NameRef{
			Name: "test-instance-ref",
		},
		Name:        ClusterName,
		Namespace:   "test-namespace",
		Labels:      Labels,
		Annotations: Annotations,
		ClusterSpec: generated.ClusterSpec{
			Description:     "test-description",
			NamespaceScoped: ptr.To(true),
			Data: generated.ClusterData{
				Size:                "medium",
				AutoUpgradeDisabled: ptr.To(AutoUpgradeDisabled),
				Kustomization:       KustomizationYAML,
				AppReplication:      ptr.To(AppReplication),
				TargetVersion:       "v1.2.3",
				RedisTunneling:      ptr.To(RedisTunneling),
			},
		},
		EnableInClusterKubeConfig: true,
		KubeConfigSecretRef: v1alpha1.SecretRef{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		RemoveAgentResourcesOnDestroy: true,
	}

	ArgocdCluster = &argocdv1.Cluster{
		Name:      ClusterName,
		Namespace: "test-namespace",
		Data: &argocdv1.ClusterData{
			Namespace:           "test-namespace",
			NamespaceScoped:     true,
			Labels:              Labels,
			Annotations:         Annotations,
			Size:                argocdv1.ClusterSize_CLUSTER_SIZE_MEDIUM,
			AutoUpgradeDisabled: &AutoUpgradeDisabled,
			Kustomization:       KustomizationPB,
			AppReplication:      &AppReplication,
			TargetVersion:       "v1.2.3",
			RedisTunneling:      &RedisTunneling,
		},
		Description:     "test-description",
		NamespaceScoped: true,
		AgentState:      ArgocdAgentState,
		HealthStatus: &health.Status{
			Code:    health.StatusCode_STATUS_CODE_HEALTHY,
			Message: "Cluster is healthy",
		},
		ReconciliationStatus: &reconciliation.Status{
			Code:    reconciliation.StatusCode_STATUS_CODE_SUCCESSFUL,
			Message: "Cluster is reconciled",
		},
	}

	AkuityCluster = akuitytypes.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "argocd.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        ClusterName,
			Namespace:   "test-namespace",
			Labels:      Labels,
			Annotations: Annotations,
		},
		Spec: akuitytypes.ClusterSpec{
			Description:     "test-description",
			NamespaceScoped: true,
			Data: akuitytypes.ClusterData{
				Size:                akuitytypes.ClusterSize("medium"),
				AutoUpgradeDisabled: &AutoUpgradeDisabled,
				Kustomization:       Kustomization,
				AppReplication:      &AppReplication,
				TargetVersion:       "v1.2.3",
				RedisTunneling:      &RedisTunneling,
			},
		},
	}

	ArgocdAgentHealthStatuses = map[string]*health.AgentHealthStatus{
		"agent1": {
			Status:  health.TenantPhase_TENANT_PHASE_HEALTHY,
			Message: "Agent 1 is healthy",
		},
		"agent2": {
			Status:  health.TenantPhase_TENANT_PHASE_DEGRADED,
			Message: "Agent 2 is degraded",
		},
	}

	CrossplaneClusterObservationAgentHealthStatuses = map[string]v1alpha1.ClusterObservationAgentHealthStatus{
		"agent1": {
			Code:    int32(health.TenantPhase_TENANT_PHASE_HEALTHY),
			Message: "Agent 1 is healthy",
		},
		"agent2": {
			Code:    int32(health.TenantPhase_TENANT_PHASE_DEGRADED),
			Message: "Agent 2 is degraded",
		},
	}

	ArgocdAgentState = &argocdv1.AgentState{
		Version:       "1.0.0",
		ArgoCdVersion: "2.0.0",
		Status: &health.AgentAggregatedHealthResponse{
			Healthy: map[string]*health.AgentHealthStatus{
				"agent1": {
					Status:  health.TenantPhase_TENANT_PHASE_HEALTHY,
					Message: "Agent 1 is healthy",
				},
			},
			Progressing: nil,
			Degraded: map[string]*health.AgentHealthStatus{
				"agent2": {
					Status:  health.TenantPhase_TENANT_PHASE_DEGRADED,
					Message: "Agent 2 is degraded",
				},
			},
			Unknown: nil,
		},
	}

	CrossplaneClusterObservationAgentState = v1alpha1.ClusterObservationAgentState{
		Version:       "1.0.0",
		ArgoCdVersion: "2.0.0",
		Statuses:      CrossplaneClusterObservationAgentHealthStatuses,
	}
)
