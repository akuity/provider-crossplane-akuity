package types

import (
	"fmt"
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"

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
	case argocdv1.ClusterSize_CLUSTER_SIZE_AUTO:
		agentSize = "auto"

	default:
		agentSize = "unspecified"
	}

	return v1alpha1.ClusterObservation{
		ID:                  cluster.GetId(),
		Name:                cluster.GetName(),
		Description:         cluster.GetDescription(),
		Namespace:           cluster.GetData().GetNamespace(),
		NamespaceScoped:     cluster.GetData().GetNamespaceScoped(),
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

func AkuityAPIToCrossplaneManagedClusterConfig(managedClusterConfig *argocdv1.ManagedClusterConfig) *generated.ManagedClusterConfig {
	if managedClusterConfig == nil {
		return nil
	}

	return &generated.ManagedClusterConfig{
		SecretName: managedClusterConfig.GetSecretName(),
		SecretKey:  managedClusterConfig.GetSecretKey(),
	}
}

func CrossplaneToAkuityAPIManagedClusterConfig(managedClusterConfig *generated.ManagedClusterConfig) *akuitytypes.ManagedClusterConfig {
	if managedClusterConfig == nil {
		return nil
	}

	return &akuitytypes.ManagedClusterConfig{
		SecretName: managedClusterConfig.SecretName,
		SecretKey:  managedClusterConfig.SecretKey,
	}
}

func AkuityAPIToCrossplaneAutoscalerConfig(autoScaleConfig *argocdv1.AutoScalerConfig) *generated.AutoScalerConfig {
	if autoScaleConfig == nil {
		return nil
	}

	crossplaneAutoscalerConfig := &generated.AutoScalerConfig{}
	if autoScaleConfig.GetApplicationController() != nil {
		applicationController := &generated.AppControllerAutoScalingConfig{}
		if autoScaleConfig.GetApplicationController().GetResourceMinimum() != nil {
			applicationController.ResourceMinimum = &generated.Resources{
				Mem: autoScaleConfig.GetApplicationController().GetResourceMinimum().GetMem(),
				Cpu: autoScaleConfig.GetApplicationController().GetResourceMinimum().GetCpu(),
			}
		}
		if autoScaleConfig.GetApplicationController().GetResourceMaximum() != nil {
			applicationController.ResourceMaximum = &generated.Resources{
				Mem: autoScaleConfig.GetApplicationController().GetResourceMaximum().GetMem(),
				Cpu: autoScaleConfig.GetApplicationController().GetResourceMaximum().GetCpu(),
			}
		}
		crossplaneAutoscalerConfig.ApplicationController = applicationController
	}
	if autoScaleConfig.GetRepoServer() != nil {
		repoServer := &generated.RepoServerAutoScalingConfig{}
		if autoScaleConfig.GetRepoServer().GetResourceMinimum() != nil {
			repoServer.ResourceMinimum = &generated.Resources{
				Mem: autoScaleConfig.GetRepoServer().GetResourceMinimum().GetMem(),
				Cpu: autoScaleConfig.GetRepoServer().GetResourceMinimum().GetCpu(),
			}
		}
		if autoScaleConfig.GetRepoServer().GetResourceMaximum() != nil {
			repoServer.ResourceMaximum = &generated.Resources{
				Mem: autoScaleConfig.GetRepoServer().GetResourceMaximum().GetMem(),
				Cpu: autoScaleConfig.GetRepoServer().GetResourceMaximum().GetCpu(),
			}
		}
		repoServer.ReplicaMinimum = autoScaleConfig.GetRepoServer().GetReplicaMinimum()
		repoServer.ReplicaMaximum = autoScaleConfig.GetRepoServer().GetReplicaMaximum()
		crossplaneAutoscalerConfig.RepoServer = repoServer
	}

	return crossplaneAutoscalerConfig
}

func AkuityAPIToCrossplaneCompatibility(compatibility *argocdv1.ClusterCompatibility) *generated.ClusterCompatibility {
	if compatibility == nil {
		return nil
	}

	return &generated.ClusterCompatibility{
		Ipv6Only: ptr.To(compatibility.GetIpv6Only()),
	}
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
	switch cluster.GetData().GetSize() {
	case argocdv1.ClusterSize_CLUSTER_SIZE_MEDIUM:
		size = "medium"
	case argocdv1.ClusterSize_CLUSTER_SIZE_LARGE:
		size = "large"
	case argocdv1.ClusterSize_CLUSTER_SIZE_AUTO:
		size = "auto"
	}

	return v1alpha1.ClusterParameters{
		InstanceID: instanceID,
		InstanceRef: v1alpha1.NameRef{
			Name: managedCluster.InstanceRef.Name,
		},
		Name:        cluster.GetName(),
		Namespace:   cluster.GetData().GetNamespace(),
		Labels:      cluster.GetData().GetLabels(),
		Annotations: cluster.GetData().GetAnnotations(),
		ClusterSpec: generated.ClusterSpec{
			Description:     cluster.GetDescription(),
			NamespaceScoped: ptr.To(cluster.GetData().GetNamespaceScoped()),
			Data: generated.ClusterData{
				Size:                            generated.ClusterSize(size),
				AutoUpgradeDisabled:             ptr.To(cluster.GetData().GetAutoUpgradeDisabled()),
				Kustomization:                   string(kustomizationYAML),
				AppReplication:                  ptr.To(cluster.GetData().GetAppReplication()),
				TargetVersion:                   cluster.GetData().GetTargetVersion(),
				RedisTunneling:                  ptr.To(cluster.GetData().GetRedisTunneling()),
				DatadogAnnotationsEnabled:       cluster.GetData().DatadogAnnotationsEnabled, //nolint:all
				EksAddonEnabled:                 cluster.GetData().EksAddonEnabled,           //nolint:all
				ManagedClusterConfig:            AkuityAPIToCrossplaneManagedClusterConfig(cluster.GetData().GetManagedClusterConfig()),
				MultiClusterK8SDashboardEnabled: cluster.GetData().MultiClusterK8SDashboardEnabled, //nolint:all
				AutoscalerConfig:                AkuityAPIToCrossplaneAutoscalerConfig(cluster.GetData().GetAutoscalerConfig()),
				Project:                         cluster.GetData().GetProject(),
				Compatibility:                   AkuityAPIToCrossplaneCompatibility(cluster.GetData().GetCompatibility()),
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

func CrossplaneToAkuityAPIAutoscalerConfig(autoscalerConfig *generated.AutoScalerConfig) *akuitytypes.AutoScalerConfig {
	if autoscalerConfig == nil {
		return nil
	}

	akuityAutoscalerConfig := &akuitytypes.AutoScalerConfig{}
	if autoscalerConfig.ApplicationController != nil {
		applicationController := &akuitytypes.AppControllerAutoScalingConfig{}
		if autoscalerConfig.ApplicationController.ResourceMinimum != nil {
			applicationController.ResourceMinimum = &akuitytypes.Resources{
				Mem: autoscalerConfig.ApplicationController.ResourceMinimum.Mem,
				Cpu: autoscalerConfig.ApplicationController.ResourceMinimum.Cpu,
			}
		}
		if autoscalerConfig.ApplicationController.ResourceMaximum != nil {
			applicationController.ResourceMaximum = &akuitytypes.Resources{
				Mem: autoscalerConfig.ApplicationController.ResourceMaximum.Mem,
				Cpu: autoscalerConfig.ApplicationController.ResourceMaximum.Cpu,
			}
		}
		akuityAutoscalerConfig.ApplicationController = applicationController
	}
	if autoscalerConfig.RepoServer != nil {
		repoServer := &akuitytypes.RepoServerAutoScalingConfig{}
		if autoscalerConfig.RepoServer.ResourceMinimum != nil {
			repoServer.ResourceMinimum = &akuitytypes.Resources{
				Mem: autoscalerConfig.RepoServer.ResourceMinimum.Mem,
				Cpu: autoscalerConfig.RepoServer.ResourceMinimum.Cpu,
			}
		}
		if autoscalerConfig.RepoServer.ResourceMaximum != nil {
			repoServer.ResourceMaximum = &akuitytypes.Resources{
				Mem: autoscalerConfig.RepoServer.ResourceMaximum.Mem,
				Cpu: autoscalerConfig.RepoServer.ResourceMaximum.Cpu,
			}
		}
		repoServer.ReplicaMinimum = autoscalerConfig.RepoServer.ReplicaMinimum
		repoServer.ReplicaMaximum = autoscalerConfig.RepoServer.ReplicaMaximum
		akuityAutoscalerConfig.RepoServer = repoServer
	}

	return akuityAutoscalerConfig
}

func CrossplaneToAkuityAPICompatibility(compatibility *generated.ClusterCompatibility) *akuitytypes.ClusterCompatibility {
	if compatibility == nil {
		return nil
	}

	return &akuitytypes.ClusterCompatibility{
		Ipv6Only: compatibility.Ipv6Only,
	}
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
				Size:                            akuitytypes.ClusterSize(cluster.ClusterSpec.Data.Size),
				AutoUpgradeDisabled:             cluster.ClusterSpec.Data.AutoUpgradeDisabled,
				Kustomization:                   kustomization,
				AppReplication:                  cluster.ClusterSpec.Data.AppReplication,
				TargetVersion:                   cluster.ClusterSpec.Data.TargetVersion,
				RedisTunneling:                  cluster.ClusterSpec.Data.RedisTunneling,
				DatadogAnnotationsEnabled:       cluster.ClusterSpec.Data.DatadogAnnotationsEnabled,
				EksAddonEnabled:                 cluster.ClusterSpec.Data.EksAddonEnabled,
				ManagedClusterConfig:            CrossplaneToAkuityAPIManagedClusterConfig(cluster.ClusterSpec.Data.ManagedClusterConfig),
				MultiClusterK8SDashboardEnabled: cluster.ClusterSpec.Data.MultiClusterK8SDashboardEnabled,
				AutoscalerConfig:                CrossplaneToAkuityAPIAutoscalerConfig(cluster.ClusterSpec.Data.AutoscalerConfig),
				Project:                         cluster.ClusterSpec.Data.Project,
				Compatibility:                   CrossplaneToAkuityAPICompatibility(cluster.ClusterSpec.Data.Compatibility),
			},
		},
	}, nil
}
