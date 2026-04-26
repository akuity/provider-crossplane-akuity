/*
Copyright 2026 The Crossplane Authors.

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

package observation

import (
	"maps"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// Cluster projects the ArgoCD-plane response into the Cluster
// AtProvider block.
func Cluster(cluster *argocdv1.Cluster) (v1alpha1.ClusterObservation, error) {
	if cluster == nil {
		return v1alpha1.ClusterObservation{}, nil
	}

	kustomizationYAML, err := marshal.PBStructToKustomizationYAML(cluster.GetData().GetKustomization())
	if err != nil {
		return v1alpha1.ClusterObservation{}, err
	}

	obs := v1alpha1.ClusterObservation{
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
		AgentSize:           clusterSizeToString(cluster.GetData().GetSize()),
		AgentState:          ClusterAgentState(cluster.GetAgentState()),
		HealthStatus: v1alpha1.ResourceStatusCode{
			Code:    int32(cluster.GetHealthStatus().GetCode()),
			Message: cluster.GetHealthStatus().GetMessage(),
		},
		ReconciliationStatus: v1alpha1.ResourceStatusCode{
			Code:    int32(cluster.GetReconciliationStatus().GetCode()),
			Message: cluster.GetReconciliationStatus().GetMessage(),
		},
	}
	// Nested mirror: groups the observed payload under one block so
	// consumers reading atProvider see the same shape as
	// spec.forProvider.clusterSpec.
	obs.ClusterSpec = crossplanetypes.ClusterSpec{
		Description:     cluster.GetDescription(),
		NamespaceScoped: boolPtrIfSet(cluster.GetData().GetNamespaceScoped()),
		Data: crossplanetypes.ClusterData{
			Size:                crossplanetypes.ClusterSize(clusterSizeToString(cluster.GetData().GetSize())),
			AutoUpgradeDisabled: cluster.GetData().AutoUpgradeDisabled,
			Kustomization:       string(kustomizationYAML),
			AppReplication:      cluster.GetData().AppReplication,
			TargetVersion:       cluster.GetData().GetTargetVersion(),
			RedisTunneling:      cluster.GetData().RedisTunneling,
			PodInheritMetadata:  cluster.GetData().PodInheritMetadata,
		},
	}
	return obs, nil
}

// boolPtrIfSet returns a pointer to b when the bool is true. Used by
// the nested ClusterSpec mirror so false-by-default proto scalars
// collapse to nil on the spec side, matching how forProvider shapes
// optional booleans.
func boolPtrIfSet(b bool) *bool {
	if !b {
		return nil
	}
	return &b
}

// ClusterAgentState projects an AgentState proto into the observed
// agent-state sub-tree. Exported so drift-detection helpers can
// rebuild the sub-tree without the full Cluster builder.
func ClusterAgentState(agentState *argocdv1.AgentState) v1alpha1.ClusterObservationAgentState {
	if agentState == nil {
		return v1alpha1.ClusterObservationAgentState{}
	}

	observedState := v1alpha1.ClusterObservationAgentState{
		Version:       agentState.GetVersion(),
		ArgoCdVersion: agentState.GetArgoCdVersion(),
	}

	if agentState.GetStatus() != nil {
		statuses := clusterAgentHealthStatuses(agentState.GetStatus().GetHealthy())
		maps.Copy(statuses, clusterAgentHealthStatuses(agentState.GetStatus().GetProgressing()))
		maps.Copy(statuses, clusterAgentHealthStatuses(agentState.GetStatus().GetDegraded()))
		maps.Copy(statuses, clusterAgentHealthStatuses(agentState.GetStatus().GetUnknown()))
		observedState.Statuses = statuses
	}

	return observedState
}

// ClusterAgentHealthStatuses converts an agent-health-status map from
// the proto shape into the curated per-agent status shape. Exposed so
// drift builders can fold a single health cohort without rebuilding
// the whole AgentState.
func ClusterAgentHealthStatuses(in map[string]*health.AgentHealthStatus) map[string]v1alpha1.ClusterObservationAgentHealthStatus {
	return clusterAgentHealthStatuses(in)
}

func clusterAgentHealthStatuses(in map[string]*health.AgentHealthStatus) map[string]v1alpha1.ClusterObservationAgentHealthStatus {
	statuses := make(map[string]v1alpha1.ClusterObservationAgentHealthStatus)
	for agentID, healthStatus := range in {
		statuses[agentID] = v1alpha1.ClusterObservationAgentHealthStatus{
			Code:    int32(healthStatus.GetStatus()),
			Message: healthStatus.GetMessage(),
		}
	}
	return statuses
}

func clusterSizeToString(s argocdv1.ClusterSize) string {
	switch s {
	case argocdv1.ClusterSize_CLUSTER_SIZE_SMALL:
		return "small"
	case argocdv1.ClusterSize_CLUSTER_SIZE_MEDIUM:
		return "medium"
	case argocdv1.ClusterSize_CLUSTER_SIZE_LARGE:
		return "large"
	case argocdv1.ClusterSize_CLUSTER_SIZE_UNSPECIFIED:
		return "unspecified"
	case argocdv1.ClusterSize_CLUSTER_SIZE_AUTO:
		return "auto"
	default:
		return "unspecified"
	}
}
