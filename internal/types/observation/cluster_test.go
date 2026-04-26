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

package observation_test

import (
	"testing"
	"time"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
)

func TestClusterAgentHealthStatuses(t *testing.T) {
	actual := observation.ClusterAgentHealthStatuses(fixtures.ArgocdAgentHealthStatuses)
	assert.Equal(t, fixtures.CrossplaneClusterObservationAgentHealthStatuses, actual)
}

func TestClusterAgentState(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneClusterObservationAgentState, observation.ClusterAgentState(fixtures.ArgocdAgentState))
}

func TestClusterAgentState_NilInput(t *testing.T) {
	assert.Equal(t, v1alpha1.ClusterObservationAgentState{}, observation.ClusterAgentState(nil))
}

func TestCluster(t *testing.T) {
	expected := v1alpha1.ClusterObservation{
		ID:                  fixtures.ArgocdCluster.GetId(),
		Name:                fixtures.ArgocdCluster.GetName(),
		Description:         fixtures.ArgocdCluster.GetDescription(),
		Namespace:           fixtures.ArgocdCluster.GetData().GetNamespace(),
		NamespaceScoped:     fixtures.ArgocdCluster.GetData().GetNamespaceScoped(),
		Labels:              fixtures.ArgocdCluster.GetData().GetLabels(),
		Annotations:         fixtures.ArgocdCluster.GetData().GetAnnotations(),
		AutoUpgradeDisabled: fixtures.ArgocdCluster.GetData().GetAutoUpgradeDisabled(),
		AppReplication:      fixtures.ArgocdCluster.GetData().GetAppReplication(),
		TargetVersion:       fixtures.ArgocdCluster.GetData().GetTargetVersion(),
		RedisTunneling:      fixtures.ArgocdCluster.GetData().GetRedisTunneling(),
		Kustomization:       fixtures.CrossplaneCluster.ClusterSpec.Data.Kustomization,
		AgentSize:           string(fixtures.CrossplaneCluster.ClusterSpec.Data.Size),
		AgentState:          fixtures.CrossplaneClusterObservationAgentState,
		HealthStatus: v1alpha1.ResourceStatusCode{
			Code:    int32(fixtures.ArgocdCluster.GetHealthStatus().GetCode()),
			Message: fixtures.ArgocdCluster.GetHealthStatus().GetMessage(),
		},
		ReconciliationStatus: v1alpha1.ResourceStatusCode{
			Code:    int32(fixtures.ArgocdCluster.GetReconciliationStatus().GetCode()),
			Message: fixtures.ArgocdCluster.GetReconciliationStatus().GetMessage(),
		},
		ClusterSpec: crossplanetypes.ClusterSpec{
			Description:     fixtures.ArgocdCluster.GetDescription(),
			NamespaceScoped: ptr.To(fixtures.ArgocdCluster.GetData().GetNamespaceScoped()),
			Data: crossplanetypes.ClusterData{
				Size:                fixtures.CrossplaneCluster.ClusterSpec.Data.Size,
				AutoUpgradeDisabled: fixtures.ArgocdCluster.GetData().AutoUpgradeDisabled,
				Kustomization:       fixtures.CrossplaneCluster.ClusterSpec.Data.Kustomization,
				AppReplication:      fixtures.ArgocdCluster.GetData().AppReplication,
				TargetVersion:       fixtures.ArgocdCluster.GetData().GetTargetVersion(),
				RedisTunneling:      fixtures.ArgocdCluster.GetData().RedisTunneling,
				PodInheritMetadata:  fixtures.ArgocdCluster.GetData().PodInheritMetadata,
			},
		},
	}

	actual, err := observation.Cluster(fixtures.ArgocdCluster)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestCluster_ClusterSpecMirrorIncludesAllClusterDataFields(t *testing.T) {
	expiry := time.Date(2026, time.April, 26, 9, 30, 0, 0, time.UTC)
	cluster := &argocdv1.Cluster{
		Description: "cluster with full data",
		Data: &argocdv1.ClusterData{
			Size:                argocdv1.ClusterSize_CLUSTER_SIZE_MEDIUM,
			AutoUpgradeDisabled: ptr.To(true),
			Kustomization:       fixtures.KustomizationPB,
			AppReplication:      ptr.To(true),
			TargetVersion:       "v1.2.3",
			RedisTunneling:      ptr.To(true),
			DirectClusterSpec: &argocdv1.DirectClusterSpec{
				ClusterType:     argocdv1.DirectClusterType_DIRECT_CLUSTER_TYPE_KARGO,
				KargoInstanceId: ptr.To("kargo-1"),
				Server:          ptr.To("https://kubernetes.example.com"),
				Organization:    ptr.To("org-1"),
				Token:           ptr.To("token-1"),
				CaData:          ptr.To("ca-1"),
			},
			DatadogAnnotationsEnabled:       ptr.To(true),
			EksAddonEnabled:                 ptr.To(false),
			ManagedClusterConfig:            &argocdv1.ManagedClusterConfig{SecretName: "cluster-secret", SecretKey: "kubeconfig"},
			MaintenanceMode:                 ptr.To(true),
			MultiClusterK8SDashboardEnabled: ptr.To(true),
			AutoscalerConfig: &argocdv1.AutoScalerConfig{
				ApplicationController: &argocdv1.AppControllerAutoScalingConfig{
					ResourceMinimum: &argocdv1.Resources{Cpu: "500m", Mem: "1Gi"},
					ResourceMaximum: &argocdv1.Resources{Cpu: "1", Mem: "2Gi"},
				},
				RepoServer: &argocdv1.RepoServerAutoScalingConfig{
					ResourceMinimum: &argocdv1.Resources{Cpu: "250m", Mem: "512Mi"},
					ResourceMaximum: &argocdv1.Resources{Cpu: "750m", Mem: "1536Mi"},
					ReplicaMinimum:  2,
					ReplicaMaximum:  5,
				},
			},
			Project:                     "project-a",
			Compatibility:               &argocdv1.ClusterCompatibility{Ipv6Only: true},
			ArgocdNotificationsSettings: &argocdv1.ClusterArgoCDNotificationsSettings{InClusterSettings: true},
			ServerSideDiffEnabled:       ptr.To(true),
			MaintenanceModeExpiry:       timestamppb.New(expiry),
			PodInheritMetadata:          ptr.To(true),
		},
	}

	actual, err := observation.Cluster(cluster)
	require.NoError(t, err)

	assert.Equal(t, crossplanetypes.ClusterData{
		Size:                "medium",
		AutoUpgradeDisabled: ptr.To(true),
		Kustomization:       fixtures.KustomizationYAML,
		AppReplication:      ptr.To(true),
		TargetVersion:       "v1.2.3",
		RedisTunneling:      ptr.To(true),
		DirectClusterSpec: &crossplanetypes.DirectClusterSpec{
			ClusterType:     "kargo",
			KargoInstanceId: ptr.To("kargo-1"),
			Server:          ptr.To("https://kubernetes.example.com"),
			Organization:    ptr.To("org-1"),
			Token:           ptr.To("token-1"),
			CaData:          ptr.To("ca-1"),
		},
		DatadogAnnotationsEnabled:       ptr.To(true),
		EksAddonEnabled:                 ptr.To(false),
		ManagedClusterConfig:            &crossplanetypes.ManagedClusterConfig{SecretName: "cluster-secret", SecretKey: "kubeconfig"},
		MaintenanceMode:                 ptr.To(true),
		MultiClusterK8SDashboardEnabled: ptr.To(true),
		AutoscalerConfig: &crossplanetypes.AutoScalerConfig{
			ApplicationController: &crossplanetypes.AppControllerAutoScalingConfig{
				ResourceMinimum: &crossplanetypes.Resources{Cpu: "500m", Mem: "1Gi"},
				ResourceMaximum: &crossplanetypes.Resources{Cpu: "1", Mem: "2Gi"},
			},
			RepoServer: &crossplanetypes.RepoServerAutoScalingConfig{
				ResourceMinimum: &crossplanetypes.Resources{Cpu: "250m", Mem: "512Mi"},
				ResourceMaximum: &crossplanetypes.Resources{Cpu: "750m", Mem: "1536Mi"},
				ReplicaMinimum:  2,
				ReplicaMaximum:  5,
			},
		},
		Project:                     "project-a",
		Compatibility:               &crossplanetypes.ClusterCompatibility{Ipv6Only: ptr.To(true)},
		ArgocdNotificationsSettings: &crossplanetypes.ClusterArgoCDNotificationsSettings{InClusterSettings: ptr.To(true)},
		ServerSideDiffEnabled:       ptr.To(true),
		MaintenanceModeExpiry:       ptr.To(expiry.Format(time.RFC3339)),
		PodInheritMetadata:          ptr.To(true),
	}, actual.ClusterSpec.Data)
}

func TestCluster_NilInput(t *testing.T) {
	actual, err := observation.Cluster(nil)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.ClusterObservation{}, actual)
}
