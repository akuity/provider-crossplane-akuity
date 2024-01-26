package types

import (
	"fmt"
	"maps"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	generated "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

func AkuityAPIToCrossplaneClusterObservation(cluster *argocdv1.Cluster) (v1alpha1.ClusterObservation, error) {
	kustomizationYAML, err := AkuityAPIKustomizationToCrossplaneKustomization(cluster.GetData().GetKustomization())
	if err != nil {
		return v1alpha1.ClusterObservation{}, err
	}

	var agentSize string
	switch cluster.GetData().GetSize() {
	case argocdv1.ClusterSize_CLUSTER_SIZE_SMALL:
		agentSize = "small"
	case argocdv1.ClusterSize_CLUSTER_SIZE_MEDIUM:
		agentSize = "medium"
	case argocdv1.ClusterSize_CLUSTER_SIZE_LARGE:
		agentSize = "large"
	case argocdv1.ClusterSize_CLUSTER_SIZE_UNSPECIFIED:
		agentSize = "unspecified"
	default:
		agentSize = "unspecified"
	}

	return v1alpha1.ClusterObservation{
		ID:                  cluster.GetId(),
		Name:                cluster.GetName(),
		Description:         cluster.GetDescription(),
		Namespace:           cluster.GetNamespace(),
		NamespaceScoped:     cluster.GetNamespaceScoped(),
		Labels:              cluster.GetData().GetLabels(),
		Annotations:         cluster.GetData().GetAnnotations(),
		AutoUpgradeDisabled: cluster.GetData().GetAutoUpgradeDisabled(),
		AppReplication:      cluster.GetData().GetAppReplication(),
		TargetVersion:       cluster.GetData().GetTargetVersion(),
		RedisTunneling:      cluster.GetData().GetRedisTunneling(),
		Kustomization:       string(kustomizationYAML),
		AgentSize:           agentSize,
		AgentState:          AkuityAPIToClusterObservationAgentState(cluster.GetAgentState()),
		HealthStatus: v1alpha1.ClusterObservationStatus{
			Code:    int32(cluster.GetHealthStatus().GetCode()),
			Message: cluster.GetHealthStatus().GetMessage(),
		},
		ReconciliationStatus: v1alpha1.ClusterObservationStatus{
			Code:    int32(cluster.GetReconciliationStatus().GetCode()),
			Message: cluster.GetReconciliationStatus().GetMessage(),
		},
	}, nil
}

func AkuityAPIToClusterObservationAgentState(agentState *argocdv1.AgentState) v1alpha1.ClusterObservationAgentState {
	if agentState == nil {
		return v1alpha1.ClusterObservationAgentState{}
	}

	observedState := v1alpha1.ClusterObservationAgentState{
		Version:       agentState.GetVersion(),
		ArgoCdVersion: agentState.GetArgoCdVersion(),
	}

	if agentState.GetStatus() != nil {
		statuses := AkuityAPIToClusterObservationAgentStateStatus(agentState.GetStatus().GetHealthy())
		maps.Copy(statuses, AkuityAPIToClusterObservationAgentStateStatus(agentState.GetStatus().GetProgressing()))
		maps.Copy(statuses, AkuityAPIToClusterObservationAgentStateStatus(agentState.GetStatus().GetDegraded()))
		maps.Copy(statuses, AkuityAPIToClusterObservationAgentStateStatus(agentState.GetStatus().GetUnknown()))
		observedState.Statuses = statuses
	}

	return observedState
}

func AkuityAPIToClusterObservationAgentStateStatus(agentHealthStatuses map[string]*health.AgentHealthStatus) map[string]v1alpha1.ClusterObservationAgentHealthStatus {
	statuses := make(map[string]v1alpha1.ClusterObservationAgentHealthStatus)
	for agentID, healthStatus := range agentHealthStatuses {
		statuses[agentID] = v1alpha1.ClusterObservationAgentHealthStatus{
			Code:    int32(healthStatus.GetStatus()),
			Message: healthStatus.GetMessage(),
		}
	}
	return statuses
}

func AkuityAPIToCrossplaneCluster(instanceID string, managedCluster v1alpha1.ClusterParameters, cluster *argocdv1.Cluster) (v1alpha1.ClusterParameters, error) {
	kustomizationYAML, err := AkuityAPIKustomizationToCrossplaneKustomization(cluster.GetData().GetKustomization())
	if err != nil {
		return v1alpha1.ClusterParameters{}, err
	}

	if len(cluster.GetData().GetLabels()) == 0 {
		cluster.Data.Labels = nil
	}

	if len(cluster.GetData().GetAnnotations()) == 0 {
		cluster.Data.Annotations = nil
	}

	size := "small"
	if cluster.GetData().GetSize() == argocdv1.ClusterSize_CLUSTER_SIZE_MEDIUM {
		size = "medium"
	} else if cluster.GetData().GetSize() == argocdv1.ClusterSize_CLUSTER_SIZE_LARGE {
		size = "large"
	}

	return v1alpha1.ClusterParameters{
		InstanceID: instanceID,
		InstanceRef: v1alpha1.NameRef{
			Name: managedCluster.InstanceRef.Name,
		},
		Name:        cluster.GetName(),
		Namespace:   cluster.GetNamespace(),
		Labels:      cluster.GetData().GetLabels(),
		Annotations: cluster.GetData().GetAnnotations(),
		ClusterSpec: generated.ClusterSpec{
			Description:     cluster.GetDescription(),
			NamespaceScoped: cluster.GetNamespaceScoped(),
			Data: generated.ClusterData{
				Size:                generated.ClusterSize(size),
				AutoUpgradeDisabled: cluster.GetData().GetAutoUpgradeDisabled(),
				Kustomization:       string(kustomizationYAML),
				AppReplication:      cluster.GetData().GetAppReplication(),
				TargetVersion:       cluster.GetData().GetTargetVersion(),
				RedisTunneling:      cluster.GetData().GetRedisTunneling(),
			},
		},
		EnableInClusterKubeConfig: managedCluster.EnableInClusterKubeConfig,
		KubeConfigSecretRef: v1alpha1.SecretRef{
			Name:      managedCluster.KubeConfigSecretRef.Name,
			Namespace: managedCluster.KubeConfigSecretRef.Namespace,
		},
		RemoveAgentResourcesOnDestroy: managedCluster.RemoveAgentResourcesOnDestroy,
	}, nil
}

func CrossplaneToAkuityAPICluster(cluster v1alpha1.ClusterParameters) (akuitytypes.Cluster, error) {
	if cluster.ClusterSpec.Data.Size == "" {
		cluster.ClusterSpec.Data.Size = "small"
	}

	kustomization := runtime.RawExtension{}
	if err := yaml.Unmarshal([]byte(cluster.ClusterSpec.Data.Kustomization), &kustomization); err != nil {
		return akuitytypes.Cluster{}, fmt.Errorf("could not unmarshal cluster Kustomization from YAML to runtime raw extension: %w", err)
	}

	return akuitytypes.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "argocd.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name,
			Namespace:   cluster.Namespace,
			Labels:      cluster.Labels,
			Annotations: cluster.Annotations,
		},
		Spec: akuitytypes.ClusterSpec{
			Description:     cluster.ClusterSpec.Description,
			NamespaceScoped: cluster.ClusterSpec.NamespaceScoped,
			Data: akuitytypes.ClusterData{
				Size:                akuitytypes.ClusterSize(cluster.ClusterSpec.Data.Size),
				AutoUpgradeDisabled: &cluster.ClusterSpec.Data.AutoUpgradeDisabled,
				Kustomization:       kustomization,
				AppReplication:      &cluster.ClusterSpec.Data.AppReplication,
				TargetVersion:       cluster.ClusterSpec.Data.TargetVersion,
				RedisTunneling:      &cluster.ClusterSpec.Data.RedisTunneling,
			},
		},
	}, nil
}
