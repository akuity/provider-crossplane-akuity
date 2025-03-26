package types_test

import (
	"testing"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	reconciliation "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"
	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	argocdtypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/argocd/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAkuityAPIToCrossplaneHostAliases(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneHostAliasesList, types.AkuityAPIToCrossplaneHostAliases(fixtures.ArgocdHostAliasesList))
}

func TestAkuityApiToCrossplaneHostAliases_EmptyList(t *testing.T) {
	assert.Nil(t, types.AkuityAPIToCrossplaneHostAliases([]*argocdv1.HostAliases{}))
}

func TestAkuityAPIToCrossplaneAppsetPolicy(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneAppsetPolicy, types.AkuityAPIToCrossplaneAppsetPolicy(fixtures.ArgocdAppsetPolicy))
}

func TestAkuityAPIToCrossplaneAppsetPolicy_NilInput(t *testing.T) {
	assert.Nil(t, types.AkuityAPIToCrossplaneAppsetPolicy(nil))
}

func TestAkuityAPIToCrossplaneAppSetDelegate(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneAppSetDelegate, types.AkuityAPIToCrossplaneAppSetDelegate(fixtures.ArgocdAppSetDelegate))
}

func TestAkuityAPIToCrossplaneAppSetDelegate_NilInput(t *testing.T) {
	assert.Nil(t, types.AkuityAPIToCrossplaneAppSetDelegate(nil))
}

func TestAkuityAPIToCrossplaneImageUpdaterDelegate(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneImageUpdateDelegate, types.AkuityAPIToCrossplaneImageUpdaterDelegate(fixtures.ArgocdImageUpdaterDelegate))
}

func TestAkuityAPIToCrossplaneImageUpdaterDelegate_NilInput(t *testing.T) {
	assert.Nil(t, types.AkuityAPIToCrossplaneImageUpdaterDelegate(nil))
}

func TestAkuityAPIToCrossplaneRepoServerDelegate(t *testing.T) {
	result := types.AkuityAPIToCrossplaneRepoServerDelegate(fixtures.ArgocdRepoServerDelegate)
	assert.Equal(t, fixtures.CrossplaneRepoServerDelegate, result)
}

func TestAkuityAPIToCrossplaneRepoServerDelegate_NilInput(t *testing.T) {
	result := types.AkuityAPIToCrossplaneRepoServerDelegate(nil)
	assert.Nil(t, result)
}

func TestAkuityAPIToCrossplaneClusterCustomization_NilInput(t *testing.T) {
	result, err := types.AkuityAPIToCrossplaneClusterCustomization(nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestAkuityAPIToCrossplaneClusterCustomization_EmptyKustomization(t *testing.T) {
	result, err := types.AkuityAPIToCrossplaneClusterCustomization(&argocdv1.ClusterCustomization{
		AutoUpgradeDisabled: false,
		AppReplication:      false,
		RedisTunneling:      false,
		Kustomization:       &structpb.Struct{},
	})
	require.NoError(t, err)
	assert.Equal(t, "{}\n", result.Kustomization)
}

func TestAkuityAPIToCrossplaneClusterCustomization(t *testing.T) {
	result, err := types.AkuityAPIToCrossplaneClusterCustomization(fixtures.ArgocdClusterCustomization)
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneClusterCustomization, result)
}

func TestAkuityAPIToCrossplaneArgoCDExtensionInstallEntry(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneInstallEntryList, types.AkuityAPIToCrossplaneArgoCDExtensionInstallEntry(fixtures.ArgocdInstallEntryList))
}

func TestAkuityAPIToCrossplaneArgoCDExtensionInstallEntry_EmptyList(t *testing.T) {
	assert.Nil(t, types.AkuityAPIToCrossplaneArgoCDExtensionInstallEntry([]*argocdv1.ArgoCDExtensionInstallEntry{}))
}

func TestAkuityAPIToCrossplaneIPAllowListEntry(t *testing.T) {
	assert.Equal(t, fixtures.CrossplaneIpAllowList, types.AkuityAPIToCrossplaneIPAllowListEntry(fixtures.ArgocdIpAllowList))
}

func TestAkuityAPIToCrossplaneIPAllowListEntry_EmptyList(t *testing.T) {
	assert.Nil(t, types.AkuityAPIToCrossplaneIPAllowListEntry([]*argocdv1.IPAllowListEntry{}))
}

func TestAkuityAPIToCrossplaneInstanceSpec(t *testing.T) {
	result, err := types.AkuityAPIToCrossplaneInstanceSpec(fixtures.ArgocdInstanceSpec)
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneInstanceSpec, result)
}

func TestAkuityAPIToCrossplaneArgoCD(t *testing.T) {
	result, err := types.AkuityAPIToCrossplaneArgoCD(fixtures.AkuityInstance)
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneInstance, result)
}

func TestAkuityAPIConfigMapToMap(t *testing.T) {
	pbConfigMap := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"key1": {
				Kind: &structpb.Value_StringValue{
					StringValue: "value1",
				},
			},
			"key2": {
				Kind: &structpb.Value_StringValue{
					StringValue: "value2",
				},
			},
		},
	}
	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	result, err := types.AkuityAPIConfigMapToMap("test-name", pbConfigMap)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestAkuityAPIConfigMapToMap_NilConfigMap(t *testing.T) {
	result, err := types.AkuityAPIConfigMapToMap("test-name", nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestAkuityAPIToCrossplaneConfigManagementPlugins_EmptyList(t *testing.T) {
	result, err := types.AkuityAPIToCrossplaneConfigManagementPlugins([]*structpb.Struct{})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestAkuityAPIToCrossplaneInstance(t *testing.T) {
	instance := &argocdv1.Instance{
		Name:        "test-instance",
		Spec:        fixtures.ArgocdInstanceSpec,
		Description: "test-description",
		Version:     "test-version",
	}

	result, err := types.AkuityAPIToCrossplaneInstance(instance, &argocdv1.ExportInstanceResponse{})
	require.NoError(t, err)
	assert.Equal(t, fixtures.CrossplaneManagedInstance, result)
}

func TestAkuityAPIToCrossplaneInstanceObservation(t *testing.T) {
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
	expectedArgoCD, _ := types.AkuityAPIToCrossplaneArgoCD(instance)

	result, err := types.AkuityAPIToCrossplaneInstanceObservation(instance, exportedInstance)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.InstanceObservation{
		ID:           "test-id",
		Name:         "test-name",
		Hostname:     "test-hostname",
		ClusterCount: 2,
		HealthStatus: v1alpha1.InstanceObservationStatus{
			Code:    200,
			Message: "OK",
		},
		ReconciliationStatus: v1alpha1.InstanceObservationStatus{
			Code:    100,
			Message: "Reconciled",
		},
		OwnerOrganizationName: "test-org",
		ArgoCD:                expectedArgoCD,
	}, result)
}

func TestCrossplaneToAkuityAPIIPAllowListEntry(t *testing.T) {
	assert.Equal(t, fixtures.AkuityIpAllowList, types.CrossplaneToAkuityAPIIPAllowListEntry(fixtures.CrossplaneIpAllowList))
}

func TestCrossplaneToAkuityAPIIPAllowListEntry_EmptyList(t *testing.T) {
	assert.Equal(t, []*akuitytypes.IPAllowListEntry{}, types.CrossplaneToAkuityAPIIPAllowListEntry([]*crossplanetypes.IPAllowListEntry{}))
}

func TestCrossplaneToAkuityAPIArgoCDExtensionInstallEntry(t *testing.T) {
	assert.Equal(t, fixtures.AkuityInstallEntryList, types.CrossplaneToAkuityAPIArgoCDExtensionInstallEntry(fixtures.CrossplaneInstallEntryList))
}

func TestCrossplaneToAkuityAPIArgoCDExtensionInstallEntry_EmptyList(t *testing.T) {
	result := types.CrossplaneToAkuityAPIArgoCDExtensionInstallEntry([]*crossplanetypes.ArgoCDExtensionInstallEntry{})
	assert.Equal(t, []*akuitytypes.ArgoCDExtensionInstallEntry{}, result)
}

func TestCrossplaneToAkuityAPIClusterCustomization(t *testing.T) {
	result, err := types.CrossplaneToAkuityAPIClusterCustomization(fixtures.CrossplaneClusterCustomization)
	require.NoError(t, err)
	assert.Equal(t, fixtures.AkuityClusterCustomization, result)
}

func TestCrossplaneToAkuityAPIClusterCustomization_NilInput(t *testing.T) {
	result, err := types.CrossplaneToAkuityAPIClusterCustomization(nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestCrossplaneToAkuityAPIRepoServerDelegate(t *testing.T) {
	assert.Equal(t, fixtures.AkuityRepoServerDelegate, types.CrossplaneToAkuityAPIRepoServerDelegate(fixtures.CrossplaneRepoServerDelegate))
}

func TestCrossplaneToAkuityAPIRepoServerDelegate_NilInput(t *testing.T) {
	assert.Nil(t, types.CrossplaneToAkuityAPIRepoServerDelegate(nil))
}

func TestCrossplaneToAkuityAPIImageUpdaterDelegate(t *testing.T) {
	assert.Equal(t, fixtures.AkuityImageUpdaterDelegate, types.CrossplaneToAkuityAPIImageUpdaterDelegate(fixtures.CrossplaneImageUpdateDelegate))
}

func TestCrossplaneToAkuityAPIImageUpdaterDelegate_NilInput(t *testing.T) {
	assert.Nil(t, types.CrossplaneToAkuityAPIImageUpdaterDelegate(nil))
}

func TestCrossplaneToAkuityAPIAppSetDelegate(t *testing.T) {
	result := types.CrossplaneToAkuityAPIAppSetDelegate(fixtures.CrossplaneAppSetDelegate)
	assert.Equal(t, fixtures.AkuityAppSetDelegate, result)
}

func TestCrossplaneToAkuityAPIAppSetDelegate_NilInput(t *testing.T) {
	assert.Nil(t, types.CrossplaneToAkuityAPIAppSetDelegate(nil))
}

func TestCrossplaneToAkuityAPIAppsetPolicy(t *testing.T) {
	assert.Equal(t, fixtures.AkuityAppsetPolicy, types.CrossplaneToAkuityAPIAppsetPolicy(fixtures.CrossplaneAppsetPolicy))
}

func TestCrossplaneToAkuityAPIAppsetPolicy_NilInput(t *testing.T) {
	assert.Nil(t, types.CrossplaneToAkuityAPIAppsetPolicy(nil))
}

func TestCrossplaneToAkuityAPIHostAliases(t *testing.T) {
	assert.Equal(t, fixtures.AkuityHostAliasesList, types.CrossplaneToAkuityAPIHostAliases(fixtures.CrossplaneHostAliasesList))
}

func TestCrossplaneToAkuityAPIConfigMap(t *testing.T) {
	name := "test-configmap"
	configMapData := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	expectedConfigMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: configMapData,
	}

	expectedConfigMapPB, err := protobuf.MarshalObjectToProtobufStruct(expectedConfigMap)
	require.NoError(t, err)

	result, err := types.CrossplaneToAkuityAPIConfigMap(name, configMapData)
	require.NoError(t, err)
	assert.Equal(t, expectedConfigMapPB, result)
}

func TestCrossplaneToAkuityAPIConfigMap_EmptyInput(t *testing.T) {
	result, err := types.CrossplaneToAkuityAPIConfigMap("test-configmap", map[string]string{})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestCrossplaneToAkuityAPIInstanceSpec(t *testing.T) {
	result, err := types.CrossplaneToAkuityAPIInstanceSpec(fixtures.CrossplaneInstanceSpec)
	require.NoError(t, err)
	assert.Equal(t, fixtures.AkuityInstanceSpec, result)
}

func TestCrossplaneToAkuityAPIArgoCD(t *testing.T) {
	name := "test-name"

	expectedArgoCD := &akuitytypes.ArgoCD{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ArgoCD",
			APIVersion: "argocd.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: akuitytypes.ArgoCDSpec{
			Description:  fixtures.CrossplaneInstance.Spec.Description,
			Version:      fixtures.CrossplaneInstance.Spec.Version,
			InstanceSpec: fixtures.AkuityInstanceSpec,
		},
	}

	expectedArgoCDPB, err := protobuf.MarshalObjectToProtobufStruct(expectedArgoCD)
	require.NoError(t, err)

	result, err := types.CrossplaneToAkuityAPIArgoCD(name, &fixtures.CrossplaneInstance)
	require.NoError(t, err)
	assert.Equal(t, expectedArgoCDPB, result)
}

func TestAkuityAPIToCrossplaneCommand(t *testing.T) {
	argocdCommand := &argocdtypes.Command{
		Command: []string{"echo"},
		Args:    []string{"Hello", "World"},
	}

	crossplaneCommand := &crossplanetypes.Command{
		Command: []string{"echo"},
		Args:    []string{"Hello", "World"},
	}

	result := types.AkuityAPIToCrossplaneCommand(argocdCommand)
	assert.Equal(t, crossplaneCommand, result)
}

func TestAkuityAPIToCrossplaneCommand_NilInput(t *testing.T) {
	assert.Nil(t, types.AkuityAPIToCrossplaneCommand(nil))
}

func TestAkuityAPIToCrossplaneDiscover(t *testing.T) {
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

	result := types.AkuityAPIToCrossplaneDiscover(argocdDiscover)
	assert.Equal(t, crossplaneDiscover, result)
}

func TestAkuityAPIToCrossplaneDiscover_NilInput(t *testing.T) {
	assert.Nil(t, types.AkuityAPIToCrossplaneDiscover(nil))
}

func TestAkuityAPIToCrossplaneDiscover_NilFind(t *testing.T) {
	argocdDiscover := &argocdtypes.Discover{
		FileName: "test-file",
		Find:     nil,
	}

	crossplaneDiscover := &crossplanetypes.Discover{
		FileName: "test-file",
		Find:     nil,
	}

	result := types.AkuityAPIToCrossplaneDiscover(argocdDiscover)
	assert.Equal(t, crossplaneDiscover, result)
}

func TestAkuityAPIToCrossplaneParameters(t *testing.T) {
	argocdParameters := &argocdtypes.Parameters{
		Static: []*argocdtypes.ParameterAnnouncement{
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

	result := types.AkuityAPIToCrossplaneParameters(argocdParameters)
	assert.Equal(t, crossplaneParameters, result)
}

func TestAkuityAPIToCrossplaneParameters_NilInput(t *testing.T) {
	assert.Nil(t, types.AkuityAPIToCrossplaneParameters(nil))
}

func TestAkuityAPIToCrossplaneParameters_NilStatic(t *testing.T) {
	argocdParameters := &argocdtypes.Parameters{
		Static: nil,
	}

	crossplaneParameters := &crossplanetypes.Parameters{
		Static: nil,
	}

	result := types.AkuityAPIToCrossplaneParameters(argocdParameters)
	assert.Equal(t, crossplaneParameters, result)
}

func TestAkuityAPIToCrossplaneParameters_NilDynamic(t *testing.T) {
	argocdParameters := &argocdtypes.Parameters{
		Dynamic: nil,
	}

	crossplaneParameters := &crossplanetypes.Parameters{
		Dynamic: nil,
	}

	result := types.AkuityAPIToCrossplaneParameters(argocdParameters)
	assert.Equal(t, crossplaneParameters, result)
}

func TestAkuityAPIKustomizationToCrossplaneKustomization(t *testing.T) {
	result, err := types.AkuityAPIKustomizationToCrossplaneKustomization(fixtures.KustomizationPB)
	require.NoError(t, err)
	assert.Equal(t, fixtures.KustomizationYAML, string(result))
}

func TestAkuityAPIKustomizationToCrossplaneKustomization_NilInput(t *testing.T) {
	result, err := types.AkuityAPIKustomizationToCrossplaneKustomization(nil)
	require.NoError(t, err)
	assert.Empty(t, string(result))
}
