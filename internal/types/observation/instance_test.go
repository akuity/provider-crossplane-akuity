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

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	reconciliation "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/utils/ptr"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	argocdtypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/argocd/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
)

func TestHostAliases(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneHostAliasesList, observation.HostAliases(fixtures.ArgocdHostAliasesList))
}

func TestHostAliases_EmptyList(t *testing.T) {
	assert.Nil(t, observation.HostAliases([]*argocdv1.HostAliases{}))
}

func TestAppsetPolicy(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneAppsetPolicy, observation.AppsetPolicy(fixtures.ArgocdAppsetPolicy))
}

func TestAppsetPolicy_NilInput(t *testing.T) {
	assert.Nil(t, observation.AppsetPolicy(nil))
}

func TestAppSetDelegate(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneAppSetDelegate, observation.AppSetDelegate(fixtures.ArgocdAppSetDelegate))
}

func TestAppSetDelegate_NilInput(t *testing.T) {
	assert.Nil(t, observation.AppSetDelegate(nil))
}

func TestImageUpdaterDelegate(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneImageUpdateDelegate, observation.ImageUpdaterDelegate(fixtures.ArgocdImageUpdaterDelegate))
}

func TestImageUpdaterDelegate_NilInput(t *testing.T) {
	assert.Nil(t, observation.ImageUpdaterDelegate(nil))
}

func TestRepoServerDelegate(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneRepoServerDelegate, observation.RepoServerDelegate(fixtures.ArgocdRepoServerDelegate))
}

func TestRepoServerDelegate_NilInput(t *testing.T) {
	assert.Nil(t, observation.RepoServerDelegate(nil))
}

func TestClusterCustomization_NilInput(t *testing.T) {
	result, err := observation.ClusterCustomization(nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestClusterCustomization_EmptyKustomization(t *testing.T) {
	result, err := observation.ClusterCustomization(&argocdv1.ClusterCustomization{
		AutoUpgradeDisabled: false,
		AppReplication:      false,
		RedisTunneling:      false,
		Kustomization:       &structpb.Struct{},
	})
	require.NoError(t, err)
	assert.Equal(t, "{}\n", result.Kustomization)
}

func TestClusterCustomization(t *testing.T) {
	result, err := observation.ClusterCustomization(fixtures.ArgocdClusterCustomization)
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneClusterCustomization, result)
}

func TestArgoCDExtensionInstallEntries(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneInstallEntryList, observation.ArgoCDExtensionInstallEntries(fixtures.ArgocdInstallEntryList))
}

func TestArgoCDExtensionInstallEntries_EmptyList(t *testing.T) {
	assert.Nil(t, observation.ArgoCDExtensionInstallEntries([]*argocdv1.ArgoCDExtensionInstallEntry{}))
}

func TestIPAllowList(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneIpAllowList, observation.IPAllowList(fixtures.ArgocdIpAllowList))
}

func TestIPAllowList_EmptyList(t *testing.T) {
	assert.Nil(t, observation.IPAllowList([]*argocdv1.IPAllowListEntry{}))
}

func TestInstanceArgoCDSpec(t *testing.T) {
	result, err := observation.InstanceArgoCDSpec(fixtures.ArgocdInstanceSpec)
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneInstanceSpec, result)
}

func TestInstanceArgoCDSpec_PropagatesAllCurrentGeneratedFields(t *testing.T) {
	spec := &argocdv1.InstanceSpec{
		ClusterCustomizationDefaults: &argocdv1.ClusterCustomization{
			ServerSideDiffEnabled: true,
		},
		Secrets: &argocdv1.SecretsManagementConfig{
			Sources: []*argocdv1.ClusterSecretMapping{
				{
					Clusters: &argocdv1.ObjectSelector{
						MatchLabels: map[string]string{"cluster": "prod"},
						MatchExpressions: []*argocdv1.LabelSelectorRequirement{
							{
								Key:      ptr.To("env"),
								Operator: ptr.To("In"),
								Values:   []string{"prod", "stage"},
							},
						},
					},
					Secrets: &argocdv1.ObjectSelector{
						MatchLabels: map[string]string{"sync": "true"},
					},
				},
			},
			Destinations: []*argocdv1.ClusterSecretMapping{
				{
					Clusters: &argocdv1.ObjectSelector{
						MatchLabels: map[string]string{"cluster": "dest"},
					},
					Secrets: &argocdv1.ObjectSelector{
						MatchExpressions: []*argocdv1.LabelSelectorRequirement{
							{
								Key:      ptr.To("team"),
								Operator: ptr.To("Exists"),
							},
						},
					},
				},
			},
		},
		MetricsIngressUsername:        ptr.To("metrics-user"),
		MetricsIngressPasswordHash:    ptr.To("metrics-hash"),
		PrivilegedNotificationCluster: ptr.To("notifications"),
		ClusterAddonsExtension: &argocdv1.ClusterAddonsExtension{
			Enabled:          true,
			AllowedUsernames: []string{"alice"},
			AllowedGroups:    []string{"admins"},
		},
		ManifestGeneration: &argocdv1.ManifestGeneration{
			Kustomize: &argocdv1.ConfigManagementToolVersions{
				DefaultVersion:     "v5.4.3",
				AdditionalVersions: []string{"v4.5.7"},
			},
		},
	}

	result, err := observation.InstanceArgoCDSpec(spec)
	require.NoError(t, err)

	assert.Equal(t, &crossplanetypes.ClusterCustomization{
		AutoUpgradeDisabled:   ptr.To(false),
		Kustomization:         "",
		AppReplication:        ptr.To(false),
		RedisTunneling:        ptr.To(false),
		ServerSideDiffEnabled: ptr.To(true),
	}, result.ClusterCustomizationDefaults)
	assert.Equal(t, &crossplanetypes.SecretsManagementConfig{
		Sources: []*crossplanetypes.ClusterSecretMapping{
			{
				Clusters: &crossplanetypes.ObjectSelector{
					MatchLabels: map[string]string{"cluster": "prod"},
					MatchExpressions: []*crossplanetypes.LabelSelectorRequirement{
						{
							Key:      ptr.To("env"),
							Operator: ptr.To("In"),
							Values:   []string{"prod", "stage"},
						},
					},
				},
				Secrets: &crossplanetypes.ObjectSelector{
					MatchLabels: map[string]string{"sync": "true"},
				},
			},
		},
		Destinations: []*crossplanetypes.ClusterSecretMapping{
			{
				Clusters: &crossplanetypes.ObjectSelector{
					MatchLabels: map[string]string{"cluster": "dest"},
				},
				Secrets: &crossplanetypes.ObjectSelector{
					MatchExpressions: []*crossplanetypes.LabelSelectorRequirement{
						{
							Key:      ptr.To("team"),
							Operator: ptr.To("Exists"),
						},
					},
				},
			},
		},
	}, result.Secrets)
	assert.Equal(t, ptr.To("metrics-user"), result.MetricsIngressUsername)
	assert.Equal(t, ptr.To("metrics-hash"), result.MetricsIngressPasswordHash)
	assert.Equal(t, ptr.To("notifications"), result.PrivilegedNotificationCluster)
	assert.Equal(t, &crossplanetypes.ClusterAddonsExtension{
		Enabled:          ptr.To(true),
		AllowedUsernames: []string{"alice"},
		AllowedGroups:    []string{"admins"},
	}, result.ClusterAddonsExtension)
	assert.Equal(t, &crossplanetypes.ManifestGeneration{
		Kustomize: &crossplanetypes.ConfigManagementToolVersions{
			DefaultVersion:     "v5.4.3",
			AdditionalVersions: []string{"v4.5.7"},
		},
	}, result.ManifestGeneration)
}

func TestInstanceArgoCD(t *testing.T) {
	result, err := observation.InstanceArgoCD(fixtures.AkuityInstance)
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneInstance, result)
}

func TestConfigMapData(t *testing.T) {
	pbConfigMap := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"key1": {Kind: &structpb.Value_StringValue{StringValue: "value1"}},
			"key2": {Kind: &structpb.Value_StringValue{StringValue: "value2"}},
		},
	}
	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	result, err := observation.ConfigMapData("test-name", pbConfigMap)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestConfigMapData_NilConfigMap(t *testing.T) {
	result, err := observation.ConfigMapData("test-name", nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestConfigManagementPlugins_EmptyList(t *testing.T) {
	result, err := observation.ConfigManagementPlugins([]*structpb.Struct{})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestInstanceSpec(t *testing.T) {
	instance := &argocdv1.Instance{
		Name:        "test-instance",
		Spec:        fixtures.ArgocdInstanceSpec,
		Description: "test-description",
		Version:     "test-version",
	}

	result, err := observation.InstanceSpec(instance, &argocdv1.ExportInstanceResponse{})
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneManagedInstance, result)
}

func TestInstance(t *testing.T) {
	instance := &argocdv1.Instance{
		Id:                    "test-id",
		Name:                  "test-name",
		Hostname:              "test-hostname",
		ClusterCount:          2,
		HealthStatus:          &health.Status{Code: 200, Message: "OK"},
		ReconciliationStatus:  &reconciliation.Status{Code: 100, Message: "Reconciled"},
		OwnerOrganizationName: "test-org",
	}

	exportedInstance := &argocdv1.ExportInstanceResponse{}
	expectedArgoCD, _ := observation.InstanceArgoCD(instance)

	result, err := observation.Instance(instance, exportedInstance)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.InstanceObservation{
		ID:           "test-id",
		Name:         "test-name",
		Hostname:     "test-hostname",
		ClusterCount: 2,
		HealthStatus: v1alpha1.ResourceStatusCode{
			Code:    200,
			Message: "OK",
		},
		ReconciliationStatus: v1alpha1.ResourceStatusCode{
			Code:    100,
			Message: "Reconciled",
		},
		OwnerOrganizationName: "test-org",
		ArgoCD:                expectedArgoCD,
	}, result)
}

func TestInstance_RedactsMetricsIngressPasswordHash(t *testing.T) {
	instance := &argocdv1.Instance{
		Spec: &argocdv1.InstanceSpec{
			MetricsIngressUsername:     ptr.To("metrics-user"),
			MetricsIngressPasswordHash: ptr.To("metrics-hash"),
		},
	}

	result, err := observation.Instance(instance, &argocdv1.ExportInstanceResponse{})
	require.NoError(t, err)

	assert.Equal(t, ptr.To("metrics-user"), result.ArgoCD.Spec.InstanceSpec.MetricsIngressUsername)
	assert.Nil(t, result.ArgoCD.Spec.InstanceSpec.MetricsIngressPasswordHash)
}

func TestCommand(t *testing.T) {
	argocdCommand := &argocdtypes.Command{
		Command: []string{"echo"},
		Args:    []string{"Hello", "World"},
	}
	crossplaneCommand := &crossplanetypes.Command{
		Command: []string{"echo"},
		Args:    []string{"Hello", "World"},
	}
	assert.Equal(t, crossplaneCommand, observation.Command(argocdCommand))
}

func TestCommand_NilInput(t *testing.T) {
	assert.Nil(t, observation.Command(nil))
}

func TestDiscover(t *testing.T) {
	argocdDiscover := &argocdtypes.Discover{
		FileName: "test-file",
		Find: &argocdtypes.Find{
			Command: []string{"echo"},
			Args:    []string{"arg1", "arg2"},
			Glob:    "test-glob",
		},
	}
	crossplaneDiscover := &crossplanetypes.Discover{
		FileName: "test-file",
		Find: &crossplanetypes.Find{
			Command: []string{"echo"},
			Args:    []string{"arg1", "arg2"},
			Glob:    "test-glob",
		},
	}
	assert.Equal(t, crossplaneDiscover, observation.Discover(argocdDiscover))
}

func TestDiscover_NilInput(t *testing.T) {
	assert.Nil(t, observation.Discover(nil))
}

func TestDiscover_NilFind(t *testing.T) {
	argocdDiscover := &argocdtypes.Discover{
		FileName: "test-file",
		Find:     nil,
	}
	crossplaneDiscover := &crossplanetypes.Discover{
		FileName: "test-file",
		Find:     nil,
	}
	assert.Equal(t, crossplaneDiscover, observation.Discover(argocdDiscover))
}

func TestParameters(t *testing.T) {
	argocdParameters := &argocdtypes.Parameters{
		Static: []*argocdtypes.ParameterAnnouncement{
			{
				Name:           "param1",
				Title:          "Parameter 1",
				Tooltip:        "Tooltip 1",
				Required:       ptr.To(true),
				ItemType:       "item1",
				CollectionType: "collection1",
				String_:        "string1",
				Array:          []string{"array1"},
				Map:            map[string]string{"key1": "value1"},
			},
			{
				Name:           "param2",
				Title:          "Parameter 2",
				Tooltip:        "Tooltip 2",
				Required:       ptr.To(false),
				ItemType:       "item2",
				CollectionType: "collection2",
				String_:        "string2",
				Array:          []string{"array2"},
				Map:            map[string]string{"key2": "value2"},
			},
		},
		Dynamic: &argocdtypes.Dynamic{
			Command: []string{"echo"},
			Args:    []string{"arg1", "arg2"},
		},
	}

	crossplaneParameters := &crossplanetypes.Parameters{
		Static: []*crossplanetypes.ParameterAnnouncement{
			{
				Name:           "param1",
				Title:          "Parameter 1",
				Tooltip:        "Tooltip 1",
				Required:       true,
				ItemType:       "item1",
				CollectionType: "collection1",
				String_:        "string1",
				Array:          []string{"array1"},
				Map:            map[string]string{"key1": "value1"},
			},
			{
				Name:           "param2",
				Title:          "Parameter 2",
				Tooltip:        "Tooltip 2",
				Required:       false,
				ItemType:       "item2",
				CollectionType: "collection2",
				String_:        "string2",
				Array:          []string{"array2"},
				Map:            map[string]string{"key2": "value2"},
			},
		},
		Dynamic: &crossplanetypes.Dynamic{
			Command: []string{"echo"},
			Args:    []string{"arg1", "arg2"},
		},
	}

	assert.Equal(t, crossplaneParameters, observation.Parameters(argocdParameters))
}

func TestParameters_NilInput(t *testing.T) {
	assert.Nil(t, observation.Parameters(nil))
}

func TestParameters_NilStatic(t *testing.T) {
	argocdParameters := &argocdtypes.Parameters{Static: nil}
	crossplaneParameters := &crossplanetypes.Parameters{Static: nil}
	assert.Equal(t, crossplaneParameters, observation.Parameters(argocdParameters))
}

func TestParameters_NilDynamic(t *testing.T) {
	argocdParameters := &argocdtypes.Parameters{Dynamic: nil}
	crossplaneParameters := &crossplanetypes.Parameters{Dynamic: nil}
	assert.Equal(t, crossplaneParameters, observation.Parameters(argocdParameters))
}
