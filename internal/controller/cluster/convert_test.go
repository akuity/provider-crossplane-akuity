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

package cluster

import (
	"encoding/json"
	"testing"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"k8s.io/utils/ptr"

	generated "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
)

func TestSpecToAPI(t *testing.T) {
	actualCluster, err := SpecToAPI(fixtures.CrossplaneCluster)
	require.NoError(t, err)
	assert.Equal(t, fixtures.AkuityCluster, actualCluster)
}

func TestAPIToSpec(t *testing.T) {
	actualCluster, err := APIToSpec(fixtures.InstanceID, fixtures.CrossplaneCluster, fixtures.ArgocdCluster)
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneCluster, actualCluster)
}

func TestSpecToAPI_PropagatesAllCurrentGeneratedClusterDataFields(t *testing.T) {
	desired := fixtures.CrossplaneCluster
	desired.ClusterSpec.Data.DirectClusterSpec = &generated.DirectClusterSpec{
		ClusterType:     "kargo",
		KargoInstanceId: ptr.To("kargo-instance"),
	}
	desired.ClusterSpec.Data.ArgocdNotificationsSettings = &generated.ClusterArgoCDNotificationsSettings{
		InClusterSettings: ptr.To(true),
	}
	desired.ClusterSpec.Data.ServerSideDiffEnabled = ptr.To(true)
	desired.ClusterSpec.Data.MaintenanceMode = ptr.To(true)
	desired.ClusterSpec.Data.MaintenanceModeExpiry = ptr.To("2026-04-26T12:00:00Z")

	wire, err := SpecToAPI(desired)
	require.NoError(t, err)

	require.NotNil(t, wire.Spec.Data.DirectClusterSpec)
	assert.Equal(t, "kargo", string(wire.Spec.Data.DirectClusterSpec.ClusterType))
	assert.Equal(t, ptr.To("kargo-instance"), wire.Spec.Data.DirectClusterSpec.KargoInstanceId)
	require.NotNil(t, wire.Spec.Data.ArgocdNotificationsSettings)
	assert.Equal(t, ptr.To(true), wire.Spec.Data.ArgocdNotificationsSettings.InClusterSettings)
	assert.Equal(t, ptr.To(true), wire.Spec.Data.ServerSideDiffEnabled)
	assert.Nil(t, wire.Spec.Data.MaintenanceMode)
	assert.Nil(t, wire.Spec.Data.MaintenanceModeExpiry)
}

func TestSpecToAPI_PassesUnknownClusterSizeToPlatform(t *testing.T) {
	desired := fixtures.CrossplaneCluster
	desired.ClusterSpec.Data.Size = generated.ClusterSize("xlarge")

	wire, err := SpecToAPI(desired)
	require.NoError(t, err)
	assert.Equal(t, "xlarge", string(wire.Spec.Data.Size))
}

func TestSpecToAPI_CanonicalizesKustomizationScaffold(t *testing.T) {
	desired := fixtures.CrossplaneCluster
	desired.ClusterSpec.Data.Kustomization = "resources:\n- namespace.yaml\n"

	wire, err := SpecToAPI(desired)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(wire.Spec.Data.Kustomization.Raw, &got))
	assert.Equal(t, "kustomize.config.k8s.io/v1beta1", got["apiVersion"])
	assert.Equal(t, "Kustomization", got["kind"])
	assert.Equal(t, []any{"namespace.yaml"}, got["resources"])
}

func TestSpecToAPI_RejectsUnknownKustomizationField(t *testing.T) {
	desired := fixtures.CrossplaneCluster
	desired.ClusterSpec.Data.Kustomization = ":bad: yaml"

	_, err := SpecToAPI(desired)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown top-level Kustomization field")
}

func TestAPIToSpec_PropagatesAllCurrentGeneratedClusterDataFields(t *testing.T) {
	apiCluster := proto.Clone(fixtures.ArgocdCluster).(*argocdv1.Cluster)
	apiCluster.Data.DirectClusterSpec = &argocdv1.DirectClusterSpec{
		ClusterType:     argocdv1.DirectClusterType_DIRECT_CLUSTER_TYPE_KARGO,
		KargoInstanceId: ptr.To("kargo-instance"),
	}
	apiCluster.Data.ArgocdNotificationsSettings = &argocdv1.ClusterArgoCDNotificationsSettings{InClusterSettings: true}
	apiCluster.Data.ServerSideDiffEnabled = ptr.To(true)

	actualCluster, err := APIToSpec(fixtures.InstanceID, fixtures.CrossplaneCluster, apiCluster)
	require.NoError(t, err)

	assert.Equal(t, &generated.DirectClusterSpec{
		ClusterType:     "kargo",
		KargoInstanceId: ptr.To("kargo-instance"),
	}, actualCluster.ClusterSpec.Data.DirectClusterSpec)
	assert.Equal(t, &generated.ClusterArgoCDNotificationsSettings{
		InClusterSettings: ptr.To(true),
	}, actualCluster.ClusterSpec.Data.ArgocdNotificationsSettings)
	assert.Equal(t, ptr.To(true), actualCluster.ClusterSpec.Data.ServerSideDiffEnabled)
}
