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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			},
		},
	}

	actual, err := observation.Cluster(fixtures.ArgocdCluster)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestCluster_NilInput(t *testing.T) {
	actual, err := observation.Cluster(nil)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.ClusterObservation{}, actual)
}
