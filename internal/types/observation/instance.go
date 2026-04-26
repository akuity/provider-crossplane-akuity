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
	"fmt"
	"strconv"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/utils/ptr"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	argocdtypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/argocd/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// ConfigMap key names are exported so callers building Apply requests
// can reference the same identifiers that observation uses when
// decoding the gateway response.
const (
	ArgocdCMKey                  = "argocd-cm"
	ArgocdImageUpdaterCMKey      = "argocd-image-updater-config"
	ArgocdImageUpdaterSSHCMKey   = "argocd-image-updater-ssh-config"
	ArgocdNotificationsCMKey     = "argocd-notifications-cm"
	ArgocdRBACCMKey              = "argocd-rbac-cm"
	ArgocdSSHKnownHostsCMKey     = "argocd-ssh-known-hosts-cm"
	ArgocdTLSCertsCMKey          = "argocd-tls-certs-cm"
	ArgocdSecretKey              = "argocd-secret"
	ArgocdApplicationSetKey      = "argocd-application-set-secret"
	ArgocdNotificationsSecretKey = "argocd-notifications-secret"
	ArgocdImageUpdaterSecretKey  = "argocd-image-updater-secret"
)

// Instance projects the Akuity instance proto + export response into
// the Instance AtProvider block.
//
//nolint:gocyclo
func Instance(instance *argocdv1.Instance, exportedInstance *argocdv1.ExportInstanceResponse) (v1alpha1.InstanceObservation, error) {
	if instance == nil {
		return v1alpha1.InstanceObservation{}, nil
	}
	argocd, err := InstanceArgoCD(instance)
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	argocdConfigMap, err := ConfigMapData(ArgocdCMKey, exportedInstance.GetArgocdConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}
	argocdImageUpdaterConfigMap, err := ConfigMapData(ArgocdImageUpdaterCMKey, exportedInstance.GetImageUpdaterConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}
	argocdImageUpdaterSSHConfigMap, err := ConfigMapData(ArgocdImageUpdaterSSHCMKey, exportedInstance.GetImageUpdaterSshConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}
	argocdNotificationsConfigMap, err := ConfigMapData(ArgocdNotificationsCMKey, exportedInstance.GetNotificationsConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}
	argocdRBACConfigMap, err := ConfigMapData(ArgocdRBACCMKey, exportedInstance.GetArgocdRbacConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}
	argocdSSHKnownHostsConfigMap, err := ConfigMapData(ArgocdSSHKnownHostsCMKey, exportedInstance.GetArgocdKnownHostsConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}
	argocdTLSCertsConfigMap, err := ConfigMapData(ArgocdTLSCertsCMKey, exportedInstance.GetArgocdTlsCertsConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	configManagementPlugins, err := ConfigManagementPlugins(exportedInstance.GetConfigManagementPlugins())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	obs := v1alpha1.InstanceObservation{
		ID:                             instance.GetId(),
		Name:                           instance.GetName(),
		Hostname:                       instance.GetHostname(),
		ClusterCount:                   instance.GetClusterCount(),
		OwnerOrganizationName:          instance.GetOwnerOrganizationName(),
		ArgoCD:                         argocd,
		Workspace:                      instance.GetWorkspaceId(),
		ArgoCDConfigMap:                argocdConfigMap,
		ArgoCDImageUpdaterConfigMap:    argocdImageUpdaterConfigMap,
		ArgoCDImageUpdaterSSHConfigMap: argocdImageUpdaterSSHConfigMap,
		ArgoCDNotificationsConfigMap:   argocdNotificationsConfigMap,
		ArgoCDRBACConfigMap:            argocdRBACConfigMap,
		ArgoCDSSHKnownHostsConfigMap:   argocdSSHKnownHostsConfigMap,
		ArgoCDTLSCertsConfigMap:        argocdTLSCertsConfigMap,
		ConfigManagementPlugins:        configManagementPlugins,
	}

	if h := instance.GetHealthStatus(); h != nil {
		obs.HealthStatus = v1alpha1.ResourceStatusCode{
			Code:    int32(h.GetCode()),
			Message: h.GetMessage(),
		}
	}
	if r := instance.GetReconciliationStatus(); r != nil {
		obs.ReconciliationStatus = v1alpha1.ResourceStatusCode{
			Code:    int32(r.GetCode()),
			Message: r.GetMessage(),
		}
	}

	return obs, nil
}

// InstanceSpec rebuilds the curated InstanceParameters from the
// Akuity gateway response + export bundle. Used by the Instance
// controller's Observe path to produce the "actual" side of drift
// comparison.
func InstanceSpec(instance *argocdv1.Instance, exportedInstance *argocdv1.ExportInstanceResponse) (v1alpha1.Instance, error) {
	argocd, err := InstanceArgoCD(instance)
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	argocdConfigMap, err := ConfigMapData(ArgocdCMKey, exportedInstance.GetArgocdConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}
	argocdImageUpdaterConfigMap, err := ConfigMapData(ArgocdImageUpdaterCMKey, exportedInstance.GetImageUpdaterConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}
	argocdImageUpdaterSSHConfigMap, err := ConfigMapData(ArgocdImageUpdaterSSHCMKey, exportedInstance.GetImageUpdaterSshConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}
	argocdNotificationsConfigMap, err := ConfigMapData(ArgocdNotificationsCMKey, exportedInstance.GetNotificationsConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}
	argocdRBACConfigMap, err := ConfigMapData(ArgocdRBACCMKey, exportedInstance.GetArgocdRbacConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}
	argocdSSHKnownHostsConfigMap, err := ConfigMapData(ArgocdSSHKnownHostsCMKey, exportedInstance.GetArgocdKnownHostsConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}
	argocdTLSCertsConfigMap, err := ConfigMapData(ArgocdTLSCertsCMKey, exportedInstance.GetArgocdTlsCertsConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}
	configManagementPlugins, err := ConfigManagementPlugins(exportedInstance.GetConfigManagementPlugins())
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	return v1alpha1.Instance{
		Spec: v1alpha1.InstanceSpec{
			ForProvider: v1alpha1.InstanceParameters{
				Name:                           instance.GetName(),
				ArgoCD:                         &argocd,
				Workspace:                      instance.GetWorkspaceId(),
				ArgoCDConfigMap:                argocdConfigMap,
				ArgoCDImageUpdaterConfigMap:    argocdImageUpdaterConfigMap,
				ArgoCDImageUpdaterSSHConfigMap: argocdImageUpdaterSSHConfigMap,
				ArgoCDNotificationsConfigMap:   argocdNotificationsConfigMap,
				ArgoCDRBACConfigMap:            argocdRBACConfigMap,
				ArgoCDSSHKnownHostsConfigMap:   argocdSSHKnownHostsConfigMap,
				ArgoCDTLSCertsConfigMap:        argocdTLSCertsConfigMap,
				ConfigManagementPlugins:        configManagementPlugins,
			},
		},
	}, nil
}

// ConfigMapData decodes the exported ConfigMap structpb.Struct into a
// plain string map. `name` is only used for error messages.
func ConfigMapData(name string, pbConfigMap *structpb.Struct) (map[string]string, error) {
	if pbConfigMap == nil {
		return nil, nil
	}
	configMap := make(map[string]string)
	if err := marshal.RemarshalTo(pbConfigMap, &configMap); err != nil {
		return configMap, fmt.Errorf("could not marshal %s configmap from protobuf struct: %w", name, err)
	}
	return configMap, nil
}

// ConfigManagementPlugins decodes the exported CMP list into the
// curated map keyed by plugin name.
func ConfigManagementPlugins(pbConfigManagementPlugins []*structpb.Struct) (map[string]crossplanetypes.ConfigManagementPlugin, error) {
	if len(pbConfigManagementPlugins) == 0 {
		return nil, nil
	}

	configManagementPlugins := make(map[string]crossplanetypes.ConfigManagementPlugin)

	for _, pbConfigManagementPlugin := range pbConfigManagementPlugins {
		configManagementPlugin := argocdtypes.ConfigManagementPlugin{}
		if err := marshal.RemarshalTo(pbConfigManagementPlugin.AsMap(), &configManagementPlugin); err != nil {
			return configManagementPlugins, fmt.Errorf("could not marshal configmap management plugin from protobuf struct: %w", err)
		}

		configManagementPlugins[configManagementPlugin.Name] = crossplanetypes.ConfigManagementPlugin{
			Enabled: configManagementPlugin.Annotations[argocdtypes.AnnotationCMPEnabled] == "true",
			Image:   configManagementPlugin.Annotations[argocdtypes.AnnotationCMPImage],
			Spec: crossplanetypes.PluginSpec{
				Version:          configManagementPlugin.Spec.Version,
				Init:             Command(configManagementPlugin.Spec.Init),
				Generate:         Command(configManagementPlugin.Spec.Generate),
				Discover:         Discover(configManagementPlugin.Spec.Discover),
				Parameters:       Parameters(configManagementPlugin.Spec.Parameters),
				PreserveFileMode: ptr.Deref(configManagementPlugin.Spec.PreserveFileMode, false),
			},
		}
	}

	return configManagementPlugins, nil
}

// InstanceArgoCD extracts the ArgoCD sub-tree from the Akuity instance
// proto. Called by Instance/InstanceSpec; exposed so callers that only
// need the inner ArgoCD shape can skip the outer observation build.
func InstanceArgoCD(instance *argocdv1.Instance) (crossplanetypes.ArgoCD, error) {
	instanceSpec, err := InstanceArgoCDSpec(instance.GetSpec())
	if err != nil {
		return crossplanetypes.ArgoCD{}, err
	}

	return crossplanetypes.ArgoCD{
		Spec: crossplanetypes.ArgoCDSpec{
			Description:  instance.GetDescription(),
			Version:      instance.GetVersion(),
			InstanceSpec: instanceSpec,
		},
	}, nil
}

// InstanceArgoCDSpec decodes the nested InstanceSpec proto into the
// curated shape. Every field-level sub-helper is exported so the
// controller's late-init path can pluck individual sub-trees.
func InstanceArgoCDSpec(instanceSpec *argocdv1.InstanceSpec) (crossplanetypes.InstanceSpec, error) {
	if instanceSpec == nil {
		return crossplanetypes.InstanceSpec{}, nil
	}

	clusterCustomization, err := ClusterCustomization(instanceSpec.GetClusterCustomizationDefaults())
	if err != nil {
		return crossplanetypes.InstanceSpec{}, err
	}

	return crossplanetypes.InstanceSpec{
		IpAllowList:                     IPAllowList(instanceSpec.GetIpAllowList()),
		Subdomain:                       instanceSpec.GetSubdomain(),
		DeclarativeManagementEnabled:    ptr.To(instanceSpec.GetDeclarativeManagementEnabled()),
		Extensions:                      ArgoCDExtensionInstallEntries(instanceSpec.GetExtensions()),
		ClusterCustomizationDefaults:    clusterCustomization,
		ImageUpdaterEnabled:             ptr.To(instanceSpec.GetImageUpdaterEnabled()),
		BackendIpAllowListEnabled:       ptr.To(instanceSpec.GetBackendIpAllowListEnabled()),
		RepoServerDelegate:              RepoServerDelegate(instanceSpec.GetRepoServerDelegate()),
		AuditExtensionEnabled:           ptr.To(instanceSpec.GetAuditExtensionEnabled()),
		SyncHistoryExtensionEnabled:     ptr.To(instanceSpec.GetSyncHistoryExtensionEnabled()),
		CrossplaneExtension:             CrossplaneExtension(instanceSpec.GetCrossplaneExtension()),
		ImageUpdaterDelegate:            ImageUpdaterDelegate(instanceSpec.GetImageUpdaterDelegate()),
		AppSetDelegate:                  AppSetDelegate(instanceSpec.GetAppSetDelegate()),
		AssistantExtensionEnabled:       ptr.To(instanceSpec.GetAssistantExtensionEnabled()),
		AppsetPolicy:                    AppsetPolicy(instanceSpec.GetAppsetPolicy()),
		HostAliases:                     HostAliases(instanceSpec.GetHostAliases()),
		AgentPermissionsRules:           AgentPermissionsRules(instanceSpec.GetAgentPermissionsRules()),
		Fqdn:                            instanceSpec.GetFqdn(),
		MultiClusterK8SDashboardEnabled: ptr.To(instanceSpec.GetMultiClusterK8SDashboardEnabled()),
		AkuityIntelligenceExtension:     AkuityIntelligenceExtension(instanceSpec.GetAkuityIntelligenceExtension()),
		ImageUpdaterVersion:             instanceSpec.GetImageUpdaterVersion(),
		CustomDeprecatedApis:            CustomDeprecatedApis(instanceSpec.GetCustomDeprecatedApis()),
		KubeVisionConfig:                KubeVisionConfig(instanceSpec.GetKubeVisionConfig()),
		AppInAnyNamespaceConfig:         AppInAnyNamespaceConfig(instanceSpec.GetAppInAnyNamespaceConfig()),
		Basepath:                        instanceSpec.GetBasepath(),
		AppsetProgressiveSyncsEnabled:   ptr.To(instanceSpec.GetAppsetProgressiveSyncsEnabled()),
		Secrets:                         SecretsManagementConfig(instanceSpec.GetSecrets()),
		AppsetPlugins:                   AppsetPlugins(instanceSpec.GetAppsetPlugins()),
		ApplicationSetExtension:         ApplicationSetExtension(instanceSpec.GetApplicationSetExtension()),
		AppReconciliationsRateLimiting:  AppReconciliationsRateLimiting(instanceSpec.GetAppReconciliationsRateLimiting()),
		MetricsIngressUsername:          instanceSpec.MetricsIngressUsername,
		MetricsIngressPasswordHash:      instanceSpec.MetricsIngressPasswordHash,
		PrivilegedNotificationCluster:   instanceSpec.PrivilegedNotificationCluster,
		ClusterAddonsExtension:          ClusterAddonsExtension(instanceSpec.GetClusterAddonsExtension()),
		ManifestGeneration:              ManifestGeneration(instanceSpec.GetManifestGeneration()),
	}, nil
}

// IPAllowList decodes the allow-list entries, returning nil for an
// empty list so callers can round-trip nil and empty via EquateEmpty.
func IPAllowList(ipAllowList []*argocdv1.IPAllowListEntry) []*crossplanetypes.IPAllowListEntry {
	if len(ipAllowList) == 0 {
		return nil
	}
	out := make([]*crossplanetypes.IPAllowListEntry, 0, len(ipAllowList))
	for _, i := range ipAllowList {
		out = append(out, &crossplanetypes.IPAllowListEntry{
			Ip:          i.GetIp(),
			Description: i.GetDescription(),
		})
	}
	return out
}

// ArgoCDExtensionInstallEntries decodes the extension list.
func ArgoCDExtensionInstallEntries(installEntryList []*argocdv1.ArgoCDExtensionInstallEntry) []*crossplanetypes.ArgoCDExtensionInstallEntry {
	if len(installEntryList) == 0 {
		return nil
	}
	out := make([]*crossplanetypes.ArgoCDExtensionInstallEntry, 0, len(installEntryList))
	for _, i := range installEntryList {
		out = append(out, &crossplanetypes.ArgoCDExtensionInstallEntry{
			Id:      i.GetId(),
			Version: i.GetVersion(),
		})
	}
	return out
}

// ClusterCustomization decodes the ClusterCustomization proto into
// the curated shape. Kustomization bytes are re-rendered into YAML
// for display in the managed resource.
func ClusterCustomization(clusterCustomization *argocdv1.ClusterCustomization) (*crossplanetypes.ClusterCustomization, error) {
	if clusterCustomization == nil {
		return nil, nil
	}
	kustomizationYAML, err := marshal.PBStructToKustomizationYAML(clusterCustomization.GetKustomization())
	if err != nil {
		return nil, err
	}
	return &crossplanetypes.ClusterCustomization{
		AutoUpgradeDisabled:   ptr.To(clusterCustomization.GetAutoUpgradeDisabled()),
		Kustomization:         string(kustomizationYAML),
		AppReplication:        ptr.To(clusterCustomization.GetAppReplication()),
		RedisTunneling:        ptr.To(clusterCustomization.GetRedisTunneling()),
		ServerSideDiffEnabled: ptr.To(clusterCustomization.GetServerSideDiffEnabled()),
	}, nil
}

// SecretsManagementConfig decodes the secret sync mapping config.
func SecretsManagementConfig(in *argocdv1.SecretsManagementConfig) *crossplanetypes.SecretsManagementConfig {
	if in == nil {
		return nil
	}
	return &crossplanetypes.SecretsManagementConfig{
		Sources:      ClusterSecretMappings(in.GetSources()),
		Destinations: ClusterSecretMappings(in.GetDestinations()),
	}
}

// ClusterSecretMappings decodes a list of cluster/secret selector pairs.
func ClusterSecretMappings(in []*argocdv1.ClusterSecretMapping) []*crossplanetypes.ClusterSecretMapping {
	if len(in) == 0 {
		return nil
	}
	out := make([]*crossplanetypes.ClusterSecretMapping, 0, len(in))
	for _, item := range in {
		out = append(out, ClusterSecretMapping(item))
	}
	return out
}

// ClusterSecretMapping decodes one cluster/secret selector pair.
func ClusterSecretMapping(in *argocdv1.ClusterSecretMapping) *crossplanetypes.ClusterSecretMapping {
	if in == nil {
		return nil
	}
	return &crossplanetypes.ClusterSecretMapping{
		Clusters: ObjectSelector(in.GetClusters()),
		Secrets:  ObjectSelector(in.GetSecrets()),
	}
}

// ObjectSelector decodes a Kubernetes label selector shape.
func ObjectSelector(in *argocdv1.ObjectSelector) *crossplanetypes.ObjectSelector {
	if in == nil {
		return nil
	}
	return &crossplanetypes.ObjectSelector{
		MatchLabels:      in.GetMatchLabels(),
		MatchExpressions: LabelSelectorRequirements(in.GetMatchExpressions()),
	}
}

// LabelSelectorRequirements decodes label selector requirements.
func LabelSelectorRequirements(in []*argocdv1.LabelSelectorRequirement) []*crossplanetypes.LabelSelectorRequirement {
	if len(in) == 0 {
		return nil
	}
	out := make([]*crossplanetypes.LabelSelectorRequirement, 0, len(in))
	for _, item := range in {
		out = append(out, LabelSelectorRequirement(item))
	}
	return out
}

// LabelSelectorRequirement decodes one label selector requirement.
func LabelSelectorRequirement(in *argocdv1.LabelSelectorRequirement) *crossplanetypes.LabelSelectorRequirement {
	if in == nil {
		return nil
	}
	return &crossplanetypes.LabelSelectorRequirement{
		Key:      in.Key,
		Operator: in.Operator,
		Values:   in.GetValues(),
	}
}

// ClusterAddonsExtension decodes cluster-addons extension settings.
func ClusterAddonsExtension(in *argocdv1.ClusterAddonsExtension) *crossplanetypes.ClusterAddonsExtension {
	if in == nil {
		return nil
	}
	return &crossplanetypes.ClusterAddonsExtension{
		Enabled:          ptr.To(in.GetEnabled()),
		AllowedUsernames: in.GetAllowedUsernames(),
		AllowedGroups:    in.GetAllowedGroups(),
	}
}

// ManifestGeneration decodes manifest generation settings.
func ManifestGeneration(in *argocdv1.ManifestGeneration) *crossplanetypes.ManifestGeneration {
	if in == nil {
		return nil
	}
	return &crossplanetypes.ManifestGeneration{
		Kustomize: ConfigManagementToolVersions(in.GetKustomize()),
	}
}

// ConfigManagementToolVersions decodes config management tool version settings.
func ConfigManagementToolVersions(in *argocdv1.ConfigManagementToolVersions) *crossplanetypes.ConfigManagementToolVersions {
	if in == nil {
		return nil
	}
	return &crossplanetypes.ConfigManagementToolVersions{
		DefaultVersion:     in.GetDefaultVersion(),
		AdditionalVersions: in.GetAdditionalVersions(),
	}
}

// RepoServerDelegate decodes the repo-server delegate proto.
func RepoServerDelegate(d *argocdv1.RepoServerDelegate) *crossplanetypes.RepoServerDelegate {
	if d == nil {
		return nil
	}
	return &crossplanetypes.RepoServerDelegate{
		ControlPlane: ptr.To(d.GetControlPlane()),
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: d.GetManagedCluster().GetClusterName(),
		},
	}
}

// CrossplaneExtension decodes the Crossplane-extension proto.
func CrossplaneExtension(ext *argocdv1.CrossplaneExtension) *crossplanetypes.CrossplaneExtension {
	if ext == nil {
		return nil
	}
	resources := make([]*crossplanetypes.CrossplaneExtensionResource, 0, len(ext.GetResources()))
	for _, r := range ext.GetResources() {
		resources = append(resources, &crossplanetypes.CrossplaneExtensionResource{Group: r.GetGroup()})
	}
	return &crossplanetypes.CrossplaneExtension{Resources: resources}
}

// ImageUpdaterDelegate decodes the image-updater delegate proto.
func ImageUpdaterDelegate(d *argocdv1.ImageUpdaterDelegate) *crossplanetypes.ImageUpdaterDelegate {
	if d == nil {
		return nil
	}
	return &crossplanetypes.ImageUpdaterDelegate{
		ControlPlane: ptr.To(d.GetControlPlane()),
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: d.GetManagedCluster().GetClusterName(),
		},
	}
}

// AppSetDelegate decodes the applicationset delegate proto.
func AppSetDelegate(d *argocdv1.AppSetDelegate) *crossplanetypes.AppSetDelegate {
	if d == nil {
		return nil
	}
	return &crossplanetypes.AppSetDelegate{
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: d.GetManagedCluster().GetClusterName(),
		},
	}
}

// AppsetPolicy decodes the applicationset-policy proto.
func AppsetPolicy(p *argocdv1.AppsetPolicy) *crossplanetypes.AppsetPolicy {
	if p == nil {
		return nil
	}
	return &crossplanetypes.AppsetPolicy{
		Policy:         p.GetPolicy(),
		OverridePolicy: ptr.To(p.GetOverridePolicy()),
	}
}

// HostAliases decodes the host-alias list.
func HostAliases(list []*argocdv1.HostAliases) []*crossplanetypes.HostAliases {
	if len(list) == 0 {
		return nil
	}
	out := make([]*crossplanetypes.HostAliases, 0, len(list))
	for _, h := range list {
		out = append(out, &crossplanetypes.HostAliases{
			Ip:        h.GetIp(),
			Hostnames: h.GetHostnames(),
		})
	}
	return out
}

// AgentPermissionsRules decodes the agent-permission-rules list.
func AgentPermissionsRules(rules []*argocdv1.AgentPermissionsRule) []*crossplanetypes.AgentPermissionsRule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]*crossplanetypes.AgentPermissionsRule, 0, len(rules))
	for _, rule := range rules {
		apiGroups := append([]string{}, rule.GetApiGroups()...)
		resources := append([]string{}, rule.GetResources()...)
		verbs := append([]string{}, rule.GetVerbs()...)
		out = append(out, &crossplanetypes.AgentPermissionsRule{
			ApiGroups: apiGroups,
			Resources: resources,
			Verbs:     verbs,
		})
	}
	return out
}

// AkuityIntelligenceExtension decodes the Akuity-intelligence proto.
func AkuityIntelligenceExtension(in *argocdv1.AkuityIntelligenceExtension) *crossplanetypes.AkuityIntelligenceExtension {
	if in == nil {
		return nil
	}
	return &crossplanetypes.AkuityIntelligenceExtension{
		Enabled:                  ptr.To(in.GetEnabled()),
		AllowedUsernames:         in.GetAllowedUsernames(),
		AllowedGroups:            in.GetAllowedGroups(),
		AiSupportEngineerEnabled: ptr.To(in.GetAiSupportEngineerEnabled()),
		ModelVersion:             in.GetModelVersion(),
	}
}

// CustomDeprecatedApis decodes the deprecated-API list.
func CustomDeprecatedApis(list []*argocdv1.CustomDeprecatedAPI) []*crossplanetypes.CustomDeprecatedAPI {
	if len(list) == 0 {
		return nil
	}
	out := make([]*crossplanetypes.CustomDeprecatedAPI, 0, len(list))
	for _, c := range list {
		out = append(out, &crossplanetypes.CustomDeprecatedAPI{
			ApiVersion:                     c.GetApiVersion(),
			NewApiVersion:                  c.GetNewApiVersion(),
			DeprecatedInKubernetesVersion:  c.GetDeprecatedInKubernetesVersion(),
			UnavailableInKubernetesVersion: c.GetUnavailableInKubernetesVersion(),
		})
	}
	return out
}

// KubeVisionConfig decodes the KubeVision proto.
func KubeVisionConfig(in *argocdv1.KubeVisionConfig) *crossplanetypes.KubeVisionConfig {
	if in == nil {
		return nil
	}
	return &crossplanetypes.KubeVisionConfig{
		CveScanConfig: &crossplanetypes.CveScanConfig{
			ScanEnabled:    ptr.To(in.GetCveScanConfig().GetScanEnabled()),
			RescanInterval: in.GetCveScanConfig().GetRescanInterval(),
		},
	}
}

// AppInAnyNamespaceConfig decodes the app-in-any-namespace proto.
func AppInAnyNamespaceConfig(in *argocdv1.AppInAnyNamespaceConfig) *crossplanetypes.AppInAnyNamespaceConfig {
	if in == nil {
		return nil
	}
	return &crossplanetypes.AppInAnyNamespaceConfig{
		Enabled: ptr.To(in.GetEnabled()),
	}
}

// AppsetPlugins decodes the applicationset-plugin list.
func AppsetPlugins(list []*argocdv1.AppsetPlugins) []*crossplanetypes.AppsetPlugins {
	if len(list) == 0 {
		return nil
	}
	out := make([]*crossplanetypes.AppsetPlugins, 0, len(list))
	for _, p := range list {
		out = append(out, &crossplanetypes.AppsetPlugins{
			Name:           p.GetName(),
			Token:          p.GetToken(),
			BaseUrl:        p.GetBaseUrl(),
			RequestTimeout: p.GetRequestTimeout(),
		})
	}
	return out
}

// ApplicationSetExtension decodes the applicationset-extension proto.
func ApplicationSetExtension(in *argocdv1.ApplicationSetExtension) *crossplanetypes.ApplicationSetExtension {
	if in == nil {
		return nil
	}
	return &crossplanetypes.ApplicationSetExtension{
		Enabled: ptr.To(in.GetEnabled()),
	}
}

// AppReconciliationsRateLimiting decodes the rate-limiting proto.
func AppReconciliationsRateLimiting(in *argocdv1.AppReconciliationsRateLimiting) *crossplanetypes.AppReconciliationsRateLimiting {
	if in == nil {
		return nil
	}
	rl := &crossplanetypes.AppReconciliationsRateLimiting{}

	if in.GetBucketRateLimiting() != nil {
		bucket := in.GetBucketRateLimiting()
		rl.BucketRateLimiting = &crossplanetypes.BucketRateLimiting{
			Enabled:    ptr.To(bucket.GetEnabled()),
			BucketSize: bucket.GetBucketSize(),
			BucketQps:  bucket.GetBucketQps(),
		}
	}

	if in.GetItemRateLimiting() != nil {
		item := in.GetItemRateLimiting()
		rl.ItemRateLimiting = &crossplanetypes.ItemRateLimiting{
			Enabled:             ptr.To(item.GetEnabled()),
			FailureCooldown:     item.GetFailureCooldown(),
			BaseDelay:           item.GetBaseDelay(),
			MaxDelay:            item.GetMaxDelay(),
			BackoffFactorString: strconv.FormatFloat(float64(item.GetBackoffFactor()), 'f', -1, 32),
		}
	}

	return rl
}

// Command decodes a CMP Command.
func Command(c *argocdtypes.Command) *crossplanetypes.Command {
	if c == nil {
		return nil
	}
	return &crossplanetypes.Command{
		Command: c.Command,
		Args:    c.Args,
	}
}

// Discover decodes a CMP Discover entry.
func Discover(d *argocdtypes.Discover) *crossplanetypes.Discover {
	if d == nil {
		return nil
	}
	out := &crossplanetypes.Discover{
		FileName: d.FileName,
	}
	if d.Find != nil {
		out.Find = &crossplanetypes.Find{
			Command: d.Find.Command,
			Args:    d.Find.Args,
			Glob:    d.Find.Glob,
		}
	}
	return out
}

// Parameters decodes a CMP Parameters entry.
func Parameters(p *argocdtypes.Parameters) *crossplanetypes.Parameters {
	if p == nil {
		return nil
	}
	out := crossplanetypes.Parameters{}

	if p.Static != nil {
		out.Static = make([]*crossplanetypes.ParameterAnnouncement, 0)
		for _, static := range p.Static {
			out.Static = append(out.Static, &crossplanetypes.ParameterAnnouncement{
				Name:           static.Name,
				Title:          static.Title,
				Tooltip:        static.Tooltip,
				Required:       ptr.Deref(static.Required, false),
				ItemType:       static.ItemType,
				CollectionType: static.CollectionType,
				String_:        static.String_,
				Array:          static.Array,
				Map:            static.Map,
			})
		}
	}

	if p.Dynamic != nil {
		out.Dynamic = &crossplanetypes.Dynamic{
			Command: p.Dynamic.Command,
			Args:    p.Dynamic.Args,
		}
	}

	return &out
}
