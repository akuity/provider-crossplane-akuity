package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
)

func TestCrossplaneToAkuityAPICluster(t *testing.T) {
	actualCluster, err := types.CrossplaneToAkuityAPICluster(fixtures.CrossplaneCluster)
	require.NoError(t, err)
	assert.Equal(t, fixtures.AkuityCluster, actualCluster)
}

func TestAkuityAPIToCrossplaneCluster(t *testing.T) {
	actualCluster, err := types.AkuityAPIToCrossplaneCluster(fixtures.InstanceID, fixtures.CrossplaneCluster, fixtures.ArgocdCluster)
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneCluster, actualCluster)
}

func TestAkuityAPIToClusterObservationAgentStateStatus(t *testing.T) {
	actualStatuses := types.AkuityAPIToClusterObservationAgentStateStatus(fixtures.ArgocdAgentHealthStatuses)
	assert.Equal(t, fixtures.CrossplaneClusterObservationAgentHealthStatuses, actualStatuses)
}

func TestAkuityAPIToClusterObservationAgentState(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneClusterObservationAgentState, types.AkuityAPIToClusterObservationAgentState(fixtures.ArgocdAgentState))
}

func TestAkuityAPIToClusterObservationAgentState_NilInput(t *testing.T) {
	assert.Equal(t, v1alpha1.ClusterObservationAgentState{}, types.AkuityAPIToClusterObservationAgentState(nil))
}

func TestAkuityAPIToCrossplaneClusterObservation(t *testing.T) {
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
		HealthStatus: v1alpha1.ClusterObservationStatus{
			Code:    int32(fixtures.ArgocdCluster.GetHealthStatus().GetCode()),
			Message: fixtures.ArgocdCluster.GetHealthStatus().GetMessage(),
		},
		ReconciliationStatus: v1alpha1.ClusterObservationStatus{
			Code:    int32(fixtures.ArgocdCluster.GetReconciliationStatus().GetCode()),
			Message: fixtures.ArgocdCluster.GetReconciliationStatus().GetMessage(),
		},
	}

	actual, err := types.AkuityAPIToCrossplaneClusterObservation(fixtures.ArgocdCluster)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}
