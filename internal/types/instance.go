package types

import (
	"fmt"
	"strconv"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"google.golang.org/protobuf/types/known/structpb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	argocdtypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/argocd/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/protobuf"
)

const (
	ARGOCD_CM_KEY                   string = "argocd-cm"
	ARGOCD_IMAGE_UPDATER_CM_KEY     string = "argocd-image-updater-config"
	ARGOCD_IMAGE_UPDATER_SSH_CM_KEY string = "argocd-image-updater-ssh-config"
	ARGOCD_NOTIFICATIONS_CM_KEY     string = "argocd-notifications-cm"
	ARGOCD_RBAC_CM_KEY              string = "argocd-rbac-cm"
	ARGOCD_SSH_KNOWN_HOSTS_CM_KEY   string = "argocd-ssh-known-hosts-cm"
	ARGOCD_TLS_CERTS_CM_KEY         string = "argocd-tls-certs-cm"

	ARGOCD_SECRET_KEY                 string = "argocd-secret"
	ARGOCD_APPLICATION_SET_SECRET_KEY string = "argocd-application-set-secret"
	ARGOCD_NOTIFICATIONS_SECRET_KEY   string = "argocd-notifications-secret"
	ARGOCD_IMAGE_UPDATER_SECRET_KEY   string = "argocd-image-updater-secret"
)

//nolint:gocyclo
func AkuityAPIToCrossplaneInstanceObservation(instance *argocdv1.Instance, exportedInstance *argocdv1.ExportInstanceResponse) (v1alpha1.InstanceObservation, error) {
	argocd, err := AkuityAPIToCrossplaneArgoCD(instance)
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	argocdConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_CM_KEY, exportedInstance.GetArgocdConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	argocdImageUpdaterConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_IMAGE_UPDATER_CM_KEY, exportedInstance.GetImageUpdaterConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	argocdImageUpdaterSSHConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_IMAGE_UPDATER_SSH_CM_KEY, exportedInstance.GetImageUpdaterSshConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	argocdNotificationsConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_NOTIFICATIONS_CM_KEY, exportedInstance.GetNotificationsConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	argocdRBACConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_RBAC_CM_KEY, exportedInstance.GetArgocdRbacConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	argocdSSHKnownHostsConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_SSH_KNOWN_HOSTS_CM_KEY, exportedInstance.GetArgocdKnownHostsConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	argocdTLSCertsConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_TLS_CERTS_CM_KEY, exportedInstance.GetArgocdTlsCertsConfigmap())
	if err != nil {
		return v1alpha1.InstanceObservation{}, err
	}

	configManagementPlugins, err := AkuityAPIToCrossplaneConfigManagementPlugins(exportedInstance.GetConfigManagementPlugins())
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
		ArgoCDConfigMap:                argocdConfigMap,
		ArgoCDImageUpdaterConfigMap:    argocdImageUpdaterConfigMap,
		ArgoCDImageUpdaterSSHConfigMap: argocdImageUpdaterSSHConfigMap,
		ArgoCDNotificationsConfigMap:   argocdNotificationsConfigMap,
		ArgoCDRBACConfigMap:            argocdRBACConfigMap,
		ArgoCDSSHKnownHostsConfigMap:   argocdSSHKnownHostsConfigMap,
		ArgoCDTLSCertsConfigMap:        argocdTLSCertsConfigMap,
		ConfigManagementPlugins:        configManagementPlugins,
	}

	if instance.GetHealthStatus() != nil {
		obs.HealthStatus = v1alpha1.InstanceObservationStatus{
			Code:    int32(instance.GetHealthStatus().GetCode()),
			Message: instance.GetHealthStatus().GetMessage(),
		}
	}

	if instance.GetReconciliationStatus() != nil {
		obs.ReconciliationStatus = v1alpha1.InstanceObservationStatus{
			Code:    int32(instance.GetReconciliationStatus().GetCode()),
			Message: instance.GetReconciliationStatus().GetMessage(),
		}
	}

	return obs, nil
}

func AkuityAPIToCrossplaneInstance(instance *argocdv1.Instance, exportedInstance *argocdv1.ExportInstanceResponse) (v1alpha1.Instance, error) {
	argocd, err := AkuityAPIToCrossplaneArgoCD(instance)
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	argocdConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_CM_KEY, exportedInstance.GetArgocdConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	argocdImageUpdaterConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_IMAGE_UPDATER_CM_KEY, exportedInstance.GetImageUpdaterConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	argocdImageUpdaterSSHConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_IMAGE_UPDATER_SSH_CM_KEY, exportedInstance.GetImageUpdaterSshConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	argocdNotificationsConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_NOTIFICATIONS_CM_KEY, exportedInstance.GetNotificationsConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	argocdRBACConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_RBAC_CM_KEY, exportedInstance.GetArgocdRbacConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	argocdSSHKnownHostsConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_SSH_KNOWN_HOSTS_CM_KEY, exportedInstance.GetArgocdKnownHostsConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	argocdTLSCertsConfigMap, err := AkuityAPIConfigMapToMap(ARGOCD_TLS_CERTS_CM_KEY, exportedInstance.GetArgocdTlsCertsConfigmap())
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	configManagementPlugins, err := AkuityAPIToCrossplaneConfigManagementPlugins(exportedInstance.GetConfigManagementPlugins())
	if err != nil {
		return v1alpha1.Instance{}, err
	}

	return v1alpha1.Instance{
		Spec: v1alpha1.InstanceSpec{
			ForProvider: v1alpha1.InstanceParameters{
				Name:                           instance.GetName(),
				ArgoCD:                         &argocd,
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

func AkuityAPIToCrossplaneConfigManagementPlugins(pbConfigManagementPlugins []*structpb.Struct) (map[string]crossplanetypes.ConfigManagementPlugin, error) {
	if len(pbConfigManagementPlugins) == 0 {
		return nil, nil
	}

	configManagementPlugins := make(map[string]crossplanetypes.ConfigManagementPlugin)

	for _, pbConfigManagementPlugin := range pbConfigManagementPlugins {
		configManagementPlugin := argocdtypes.ConfigManagementPlugin{}
		err := protobuf.RemarshalObject(pbConfigManagementPlugin.AsMap(), &configManagementPlugin)
		if err != nil {
			return configManagementPlugins, fmt.Errorf("could not marshal configmap management plugin from protobuf struct: %w", err)
		}

		configManagementPlugins[configManagementPlugin.Name] = crossplanetypes.ConfigManagementPlugin{
			Enabled: configManagementPlugin.Annotations[argocdtypes.AnnotationCMPEnabled] == "true",
			Image:   configManagementPlugin.Annotations[argocdtypes.AnnotationCMPImage],
			Spec: crossplanetypes.PluginSpec{
				Version:          configManagementPlugin.Spec.Version,
				Init:             AkuityAPIToCrossplaneCommand(configManagementPlugin.Spec.Init),
				Generate:         AkuityAPIToCrossplaneCommand(configManagementPlugin.Spec.Generate),
				Discover:         AkuityAPIToCrossplaneDiscover(configManagementPlugin.Spec.Discover),
				Parameters:       AkuityAPIToCrossplaneParameters(configManagementPlugin.Spec.Parameters),
				PreserveFileMode: configManagementPlugin.Spec.PreserveFileMode,
			},
		}
	}

	return configManagementPlugins, nil
}

func AkuityAPIToCrossplaneCommand(command *argocdtypes.Command) *crossplanetypes.Command {
	if command == nil {
		return nil
	}

	return &crossplanetypes.Command{
		Command: command.Command,
		Args:    command.Args,
	}
}

func AkuityAPIToCrossplaneDiscover(discover *argocdtypes.Discover) *crossplanetypes.Discover {
	if discover == nil {
		return nil
	}

	crossplaneDiscover := &crossplanetypes.Discover{
		FileName: discover.FileName,
	}

	if discover.Find != nil {
		crossplaneDiscover.Find = &crossplanetypes.Find{
			Command: discover.Find.Command,
			Args:    discover.Find.Args,
			Glob:    discover.Find.Glob,
		}
	}

	return crossplaneDiscover
}

func AkuityAPIToCrossplaneParameters(parameters *argocdtypes.Parameters) *crossplanetypes.Parameters {
	if parameters == nil {
		return nil
	}

	crossplaneParameters := crossplanetypes.Parameters{}

	if parameters.Static != nil {
		crossplaneParameters.Static = make([]*crossplanetypes.ParameterAnnouncement, 0)

		for _, static := range parameters.Static {
			crossplaneParameters.Static = append(crossplaneParameters.Static, &crossplanetypes.ParameterAnnouncement{
				Name:           static.Name,
				Title:          static.Title,
				Tooltip:        static.Tooltip,
				Required:       static.Required,
				ItemType:       static.ItemType,
				CollectionType: static.CollectionType,
				String_:        static.String_,
				Array:          static.Array,
				Map:            static.Map,
			})
		}
	}

	if parameters.Dynamic != nil {
		crossplaneParameters.Dynamic = &crossplanetypes.Dynamic{
			Command: parameters.Dynamic.Command,
			Args:    parameters.Dynamic.Args,
		}
	}

	return &crossplaneParameters
}

func AkuityAPIConfigMapToMap(name string, pbConfigMap *structpb.Struct) (map[string]string, error) {
	if pbConfigMap == nil {
		return nil, nil
	}

	configMap := make(map[string]string)
	err := protobuf.RemarshalObject(pbConfigMap, &configMap)
	if err != nil {
		return configMap, fmt.Errorf("could not marshal %s configmap from protobuf struct: %w", name, err)
	}

	return configMap, nil
}

func AkuityAPIToCrossplaneArgoCD(instance *argocdv1.Instance) (crossplanetypes.ArgoCD, error) {
	instanceSpec, err := AkuityAPIToCrossplaneInstanceSpec(instance.GetSpec())
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

func AkuityAPIToCrossplaneInstanceSpec(instanceSpec *argocdv1.InstanceSpec) (crossplanetypes.InstanceSpec, error) {
	if instanceSpec == nil {
		return crossplanetypes.InstanceSpec{}, nil
	}

	clusterCustomization, err := AkuityAPIToCrossplaneClusterCustomization(instanceSpec.GetClusterCustomizationDefaults())
	if err != nil {
		return crossplanetypes.InstanceSpec{}, err
	}

	return crossplanetypes.InstanceSpec{
		IpAllowList:                     AkuityAPIToCrossplaneIPAllowListEntry(instanceSpec.GetIpAllowList()),
		Subdomain:                       instanceSpec.GetSubdomain(),
		DeclarativeManagementEnabled:    ptr.To(instanceSpec.GetDeclarativeManagementEnabled()),
		Extensions:                      AkuityAPIToCrossplaneArgoCDExtensionInstallEntry(instanceSpec.GetExtensions()),
		ClusterCustomizationDefaults:    clusterCustomization,
		ImageUpdaterEnabled:             ptr.To(instanceSpec.GetImageUpdaterEnabled()),
		BackendIpAllowListEnabled:       ptr.To(instanceSpec.GetBackendIpAllowListEnabled()),
		RepoServerDelegate:              AkuityAPIToCrossplaneRepoServerDelegate(instanceSpec.GetRepoServerDelegate()),
		AuditExtensionEnabled:           ptr.To(instanceSpec.GetAuditExtensionEnabled()),
		SyncHistoryExtensionEnabled:     ptr.To(instanceSpec.GetSyncHistoryExtensionEnabled()),
		CrossplaneExtension:             AkuityAPIToCrossplaneCrossplaneExtension(instanceSpec.GetCrossplaneExtension()),
		ImageUpdaterDelegate:            AkuityAPIToCrossplaneImageUpdaterDelegate(instanceSpec.GetImageUpdaterDelegate()),
		AppSetDelegate:                  AkuityAPIToCrossplaneAppSetDelegate(instanceSpec.GetAppSetDelegate()),
		AssistantExtensionEnabled:       ptr.To(instanceSpec.GetAssistantExtensionEnabled()),
		AppsetPolicy:                    AkuityAPIToCrossplaneAppsetPolicy(instanceSpec.GetAppsetPolicy()),
		HostAliases:                     AkuityAPIToCrossplaneHostAliases(instanceSpec.GetHostAliases()),
		AgentPermissionsRules:           AkuityAPIToCrossplaneAgentPermissionsRules(instanceSpec.GetAgentPermissionsRules()),
		Fqdn:                            instanceSpec.GetFqdn(),
		MultiClusterK8SDashboardEnabled: ptr.To(instanceSpec.GetMultiClusterK8SDashboardEnabled()),
		AkuityIntelligenceExtension:     AkuityAPIToCrossplaneAkuityIntelligenceExtension(instanceSpec.GetAkuityIntelligenceExtension()),
		ImageUpdaterVersion:             instanceSpec.GetImageUpdaterVersion(),
		CustomDeprecatedApis:            AkuityAPIToCrossplaneCustomDeprecatedApis(instanceSpec.GetCustomDeprecatedApis()),
		KubeVisionConfig:                AkuityAPIToCrossplaneKubeVisionConfig(instanceSpec.GetKubeVisionConfig()),
		AppInAnyNamespaceConfig:         AkuityAPIToCrossplaneAppInAnyNamespaceConfig(instanceSpec.GetAppInAnyNamespaceConfig()),
		Basepath:                        instanceSpec.GetBasepath(),
		AppsetProgressiveSyncsEnabled:   ptr.To(instanceSpec.GetAppsetProgressiveSyncsEnabled()),
		AppsetPlugins:                   AkuityAPIToCrossplaneAppsetPlugins(instanceSpec.GetAppsetPlugins()),
		ApplicationSetExtension:         AkuityAPIToCrossplaneApplicationSetExtension(instanceSpec.GetApplicationSetExtension()),
		AppReconciliationsRateLimiting:  AkuityAPIToCrossplaneAppReconciliationsRateLimiting(instanceSpec.GetAppReconciliationsRateLimiting()),
	}, nil
}

func AkuityAPIToCrossplaneIPAllowListEntry(ipAllowList []*argocdv1.IPAllowListEntry) []*crossplanetypes.IPAllowListEntry {
	if len(ipAllowList) == 0 {
		return nil
	}

	crossplaneIpAllowList := make([]*crossplanetypes.IPAllowListEntry, 0)

	for _, i := range ipAllowList {
		crossplaneIpAllowList = append(crossplaneIpAllowList, &crossplanetypes.IPAllowListEntry{
			Ip:          i.GetIp(),
			Description: i.GetDescription(),
		})
	}

	return crossplaneIpAllowList
}

func AkuityAPIToCrossplaneArgoCDExtensionInstallEntry(installEntryList []*argocdv1.ArgoCDExtensionInstallEntry) []*crossplanetypes.ArgoCDExtensionInstallEntry {
	if len(installEntryList) == 0 {
		return nil
	}

	crossplaneInstallEntryList := make([]*crossplanetypes.ArgoCDExtensionInstallEntry, 0)

	for _, i := range installEntryList {
		crossplaneInstallEntryList = append(crossplaneInstallEntryList, &crossplanetypes.ArgoCDExtensionInstallEntry{
			Id:      i.GetId(),
			Version: i.GetVersion(),
		})
	}

	return crossplaneInstallEntryList
}

func AkuityAPIKustomizationToCrossplaneKustomization(kustomization *structpb.Struct) ([]byte, error) {
	crossplaneKustomization := runtime.RawExtension{}
	crossplaneKustomizationYAML := []byte{}
	if kustomization != nil {
		err := protobuf.RemarshalObject(kustomization, &crossplaneKustomization)
		if err != nil {
			return nil, fmt.Errorf("could not marshal Kustomization from pb struct to runtime raw extension: %w", err)
		}

		crossplaneKustomizationYAML, err = yaml.JSONToYAML(crossplaneKustomization.Raw)
		if err != nil {
			return nil, fmt.Errorf("could not convert Kustomization from JSON to YAML: %w", err)
		}
	}

	return crossplaneKustomizationYAML, nil
}

func AkuityAPIToCrossplaneClusterCustomization(clusterCustomization *argocdv1.ClusterCustomization) (*crossplanetypes.ClusterCustomization, error) {
	if clusterCustomization == nil {
		return nil, nil
	}

	kustomizationYAML, err := AkuityAPIKustomizationToCrossplaneKustomization(clusterCustomization.GetKustomization())
	if err != nil {
		return nil, err
	}

	return &crossplanetypes.ClusterCustomization{
		AutoUpgradeDisabled: ptr.To(clusterCustomization.GetAutoUpgradeDisabled()),
		Kustomization:       string(kustomizationYAML),
		AppReplication:      ptr.To(clusterCustomization.GetAppReplication()),
		RedisTunneling:      ptr.To(clusterCustomization.GetRedisTunneling()),
	}, nil
}

func AkuityAPIToCrossplaneRepoServerDelegate(repoServerDelegate *argocdv1.RepoServerDelegate) *crossplanetypes.RepoServerDelegate {
	if repoServerDelegate == nil {
		return nil
	}

	return &crossplanetypes.RepoServerDelegate{
		ControlPlane: ptr.To(repoServerDelegate.GetControlPlane()),
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: repoServerDelegate.GetManagedCluster().GetClusterName(),
		},
	}
}

func AkuityAPIToCrossplaneCrossplaneExtension(crossplaneExtension *argocdv1.CrossplaneExtension) *crossplanetypes.CrossplaneExtension {
	if crossplaneExtension == nil {
		return nil
	}

	resources := make([]*crossplanetypes.CrossplaneExtensionResource, 0, len(crossplaneExtension.GetResources()))
	for _, r := range crossplaneExtension.GetResources() {
		resource := &crossplanetypes.CrossplaneExtensionResource{
			Group: r.GetGroup(),
		}
		resources = append(resources, resource)
	}
	return &crossplanetypes.CrossplaneExtension{Resources: resources}
}

func AkuityAPIToCrossplaneImageUpdaterDelegate(imageUpdaterDelegate *argocdv1.ImageUpdaterDelegate) *crossplanetypes.ImageUpdaterDelegate {
	if imageUpdaterDelegate == nil {
		return nil
	}

	return &crossplanetypes.ImageUpdaterDelegate{
		ControlPlane: ptr.To(imageUpdaterDelegate.GetControlPlane()),
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: imageUpdaterDelegate.GetManagedCluster().GetClusterName(),
		},
	}
}

func AkuityAPIToCrossplaneAppSetDelegate(appSetDelegate *argocdv1.AppSetDelegate) *crossplanetypes.AppSetDelegate {
	if appSetDelegate == nil {
		return nil
	}

	return &crossplanetypes.AppSetDelegate{
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: appSetDelegate.GetManagedCluster().GetClusterName(),
		},
	}
}

func AkuityAPIToCrossplaneAppsetPolicy(appsetPolicy *argocdv1.AppsetPolicy) *crossplanetypes.AppsetPolicy {
	if appsetPolicy == nil {
		return nil
	}

	return &crossplanetypes.AppsetPolicy{
		Policy:         appsetPolicy.GetPolicy(),
		OverridePolicy: ptr.To(appsetPolicy.GetOverridePolicy()),
	}
}

func AkuityAPIToCrossplaneHostAliases(hostAliasesList []*argocdv1.HostAliases) []*crossplanetypes.HostAliases {
	if len(hostAliasesList) == 0 {
		return nil
	}

	crossplaneHostAliasesList := make([]*crossplanetypes.HostAliases, 0)

	for _, h := range hostAliasesList {
		crossplaneHostAliasesList = append(crossplaneHostAliasesList, &crossplanetypes.HostAliases{
			Ip:        h.GetIp(),
			Hostnames: h.GetHostnames(),
		})
	}

	return crossplaneHostAliasesList
}

func AkuityAPIToCrossplaneAgentPermissionsRules(agentPermissionsRules []*argocdv1.AgentPermissionsRule) []*crossplanetypes.AgentPermissionsRule {
	if len(agentPermissionsRules) == 0 {
		return nil
	}

	crossplaneAgentPermissionsRules := make([]*crossplanetypes.AgentPermissionsRule, 0, len(agentPermissionsRules))
	for _, rule := range agentPermissionsRules {
		var apiGroups []string
		apiGroups = append(apiGroups, rule.GetApiGroups()...)
		var resources []string
		resources = append(resources, rule.GetResources()...)
		var verbs []string
		verbs = append(verbs, rule.GetVerbs()...)
		crossplaneAgentPermissionsRules = append(crossplaneAgentPermissionsRules, &crossplanetypes.AgentPermissionsRule{
			ApiGroups: apiGroups,
			Resources: resources,
			Verbs:     verbs,
		})
	}
	return crossplaneAgentPermissionsRules
}

func AkuityAPIToCrossplaneAkuityIntelligenceExtension(apiIntelligenceExtension *argocdv1.AkuityIntelligenceExtension) *crossplanetypes.AkuityIntelligenceExtension {
	if apiIntelligenceExtension == nil {
		return nil
	}

	return &crossplanetypes.AkuityIntelligenceExtension{
		Enabled:                  ptr.To(apiIntelligenceExtension.GetEnabled()),
		AllowedUsernames:         apiIntelligenceExtension.GetAllowedUsernames(),
		AllowedGroups:            apiIntelligenceExtension.GetAllowedGroups(),
		AiSupportEngineerEnabled: ptr.To(apiIntelligenceExtension.GetAiSupportEngineerEnabled()),
		ModelVersion:             apiIntelligenceExtension.GetModelVersion(),
	}
}

func AkuityAPIToCrossplaneCustomDeprecatedApis(customDeprecatedApis []*argocdv1.CustomDeprecatedAPI) []*crossplanetypes.CustomDeprecatedAPI {
	if len(customDeprecatedApis) == 0 {
		return nil
	}

	crossplaneCustomDeprecatedApis := make([]*crossplanetypes.CustomDeprecatedAPI, 0, len(customDeprecatedApis))
	for _, c := range customDeprecatedApis {
		crossplaneCustomDeprecatedApis = append(crossplaneCustomDeprecatedApis, &crossplanetypes.CustomDeprecatedAPI{
			ApiVersion:                     c.GetApiVersion(),
			NewApiVersion:                  c.GetNewApiVersion(),
			DeprecatedInKubernetesVersion:  c.GetDeprecatedInKubernetesVersion(),
			UnavailableInKubernetesVersion: c.GetUnavailableInKubernetesVersion(),
		})
	}

	return crossplaneCustomDeprecatedApis
}

func AkuityAPIToCrossplaneKubeVisionConfig(kubeVisionConfig *argocdv1.KubeVisionConfig) *crossplanetypes.KubeVisionConfig {
	if kubeVisionConfig == nil {
		return nil
	}

	return &crossplanetypes.KubeVisionConfig{
		CveScanConfig: &crossplanetypes.CveScanConfig{
			ScanEnabled:    ptr.To(kubeVisionConfig.GetCveScanConfig().GetScanEnabled()),
			RescanInterval: kubeVisionConfig.GetCveScanConfig().GetRescanInterval(),
		},
	}
}

func AkuityAPIToCrossplaneAppInAnyNamespaceConfig(appInAnyNamespaceConfig *argocdv1.AppInAnyNamespaceConfig) *crossplanetypes.AppInAnyNamespaceConfig {
	if appInAnyNamespaceConfig == nil {
		return nil
	}

	return &crossplanetypes.AppInAnyNamespaceConfig{
		Enabled: ptr.To(appInAnyNamespaceConfig.GetEnabled()),
	}
}

func AkuityAPIToCrossplaneAppsetPlugins(appsetPlugins []*argocdv1.AppsetPlugins) []*crossplanetypes.AppsetPlugins {
	if len(appsetPlugins) == 0 {
		return nil
	}

	crossplaneAppsetPlugins := make([]*crossplanetypes.AppsetPlugins, 0, len(appsetPlugins))
	for _, p := range appsetPlugins {
		crossplaneAppsetPlugins = append(crossplaneAppsetPlugins, &crossplanetypes.AppsetPlugins{
			Name:           p.GetName(),
			Token:          p.GetToken(),
			BaseUrl:        p.GetBaseUrl(),
			RequestTimeout: p.GetRequestTimeout(),
		})
	}

	return crossplaneAppsetPlugins
}

func AkuityAPIToCrossplaneApplicationSetExtension(applicationSetExtension *argocdv1.ApplicationSetExtension) *crossplanetypes.ApplicationSetExtension {
	if applicationSetExtension == nil {
		return nil
	}

	return &crossplanetypes.ApplicationSetExtension{
		Enabled: ptr.To(applicationSetExtension.GetEnabled()),
	}
}

func AkuityAPIToCrossplaneAppReconciliationsRateLimiting(appReconciliationsRateLimiting *argocdv1.AppReconciliationsRateLimiting) *crossplanetypes.AppReconciliationsRateLimiting {
	if appReconciliationsRateLimiting == nil {
		return nil
	}

	rl := &crossplanetypes.AppReconciliationsRateLimiting{}

	if appReconciliationsRateLimiting.GetBucketRateLimiting() != nil {
		bucket := appReconciliationsRateLimiting.GetBucketRateLimiting()
		rl.BucketRateLimiting = &crossplanetypes.BucketRateLimiting{
			Enabled:    ptr.To(bucket.GetEnabled()),
			BucketSize: bucket.GetBucketSize(),
			BucketQps:  bucket.GetBucketQps(),
		}
	}

	if appReconciliationsRateLimiting.GetItemRateLimiting() != nil {
		item := appReconciliationsRateLimiting.GetItemRateLimiting()
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

func CrossplaneToAkuityAPIArgoCD(name string, instance *crossplanetypes.ArgoCD) (*structpb.Struct, error) {
	instanceSpec, err := CrossplaneToAkuityAPIInstanceSpec(instance.Spec.InstanceSpec)
	if err != nil {
		return nil, err
	}

	argocd := akuitytypes.ArgoCD{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ArgoCD",
			APIVersion: "argocd.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: akuitytypes.ArgoCDSpec{
			Description:  instance.Spec.Description,
			Version:      instance.Spec.Version,
			InstanceSpec: instanceSpec,
		},
	}

	argocdPB, err := protobuf.MarshalObjectToProtobufStruct(argocd)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd spec to protobuf: %w", err)
	}

	return argocdPB, nil
}

func CrossplaneToAkuityAPIInstanceSpec(instanceSpec crossplanetypes.InstanceSpec) (akuitytypes.InstanceSpec, error) {
	clusterCustomization, err := CrossplaneToAkuityAPIClusterCustomization(instanceSpec.ClusterCustomizationDefaults)
	if err != nil {
		return akuitytypes.InstanceSpec{}, fmt.Errorf("could not build instance argocd instance spec: %w", err)
	}

	appReconciliationsRateLimiting, err := CrossplaneToAkuityAPIAppReconciliationsRateLimiting(instanceSpec.AppReconciliationsRateLimiting)
	if err != nil {
		return akuitytypes.InstanceSpec{}, fmt.Errorf("could not build instance app reconciliations rate limiting config: %w", err)
	}

	return akuitytypes.InstanceSpec{
		IpAllowList:                     CrossplaneToAkuityAPIIPAllowListEntry(instanceSpec.IpAllowList),
		Subdomain:                       instanceSpec.Subdomain,
		DeclarativeManagementEnabled:    instanceSpec.DeclarativeManagementEnabled,
		Extensions:                      CrossplaneToAkuityAPIArgoCDExtensionInstallEntry(instanceSpec.Extensions),
		ClusterCustomizationDefaults:    clusterCustomization,
		ImageUpdaterEnabled:             instanceSpec.ImageUpdaterEnabled,
		BackendIpAllowListEnabled:       instanceSpec.BackendIpAllowListEnabled,
		RepoServerDelegate:              CrossplaneToAkuityAPIRepoServerDelegate(instanceSpec.RepoServerDelegate),
		AuditExtensionEnabled:           instanceSpec.AuditExtensionEnabled,
		SyncHistoryExtensionEnabled:     instanceSpec.SyncHistoryExtensionEnabled,
		CrossplaneExtension:             CrossplaneToAkuityAPICrossplaneExtension(instanceSpec.CrossplaneExtension),
		ImageUpdaterDelegate:            CrossplaneToAkuityAPIImageUpdaterDelegate(instanceSpec.ImageUpdaterDelegate),
		AppSetDelegate:                  CrossplaneToAkuityAPIAppSetDelegate(instanceSpec.AppSetDelegate),
		AssistantExtensionEnabled:       instanceSpec.AssistantExtensionEnabled,
		AppsetPolicy:                    CrossplaneToAkuityAPIAppsetPolicy(instanceSpec.AppsetPolicy),
		HostAliases:                     CrossplaneToAkuityAPIHostAliases(instanceSpec.HostAliases),
		AgentPermissionsRules:           CrossplaneToAkuityAPIAgentPermissionsRules(instanceSpec.AgentPermissionsRules),
		Fqdn:                            instanceSpec.Fqdn,
		MultiClusterK8SDashboardEnabled: instanceSpec.MultiClusterK8SDashboardEnabled,
		AkuityIntelligenceExtension:     CrossplaneToAkuityAPIAkuityIntelligenceExtension(instanceSpec.AkuityIntelligenceExtension),
		ImageUpdaterVersion:             instanceSpec.ImageUpdaterVersion,
		CustomDeprecatedApis:            CrossplaneToAkuityAPICustomDeprecatedApis(instanceSpec.CustomDeprecatedApis),
		KubeVisionConfig:                CrossplaneToAkuityAPIKubeVisionConfig(instanceSpec.KubeVisionConfig),
		AppInAnyNamespaceConfig:         CrossplaneToAkuityAPIAppInAnyNamespaceConfig(instanceSpec.AppInAnyNamespaceConfig),
		Basepath:                        instanceSpec.Basepath,
		AppsetProgressiveSyncsEnabled:   instanceSpec.AppsetProgressiveSyncsEnabled,
		AppsetPlugins:                   CrossplaneToAkuityAPIAppsetPlugins(instanceSpec.AppsetPlugins),
		ApplicationSetExtension:         CrossplaneToAkuityAPIApplicationSetExtension(instanceSpec.ApplicationSetExtension),
		AppReconciliationsRateLimiting:  appReconciliationsRateLimiting,
	}, nil
}

func CrossplaneToAkuityAPIIPAllowListEntry(ipAllowList []*crossplanetypes.IPAllowListEntry) []*akuitytypes.IPAllowListEntry {
	AkuityIpAllowList := make([]*akuitytypes.IPAllowListEntry, 0)

	for _, i := range ipAllowList {
		AkuityIpAllowList = append(AkuityIpAllowList, &akuitytypes.IPAllowListEntry{
			Ip:          i.Ip,
			Description: i.Description,
		})
	}

	return AkuityIpAllowList
}

func CrossplaneToAkuityAPIArgoCDExtensionInstallEntry(installEntryList []*crossplanetypes.ArgoCDExtensionInstallEntry) []*akuitytypes.ArgoCDExtensionInstallEntry {
	AkuityInstallEntryList := make([]*akuitytypes.ArgoCDExtensionInstallEntry, 0)

	for _, i := range installEntryList {
		AkuityInstallEntryList = append(AkuityInstallEntryList, &akuitytypes.ArgoCDExtensionInstallEntry{
			Id:      i.Id,
			Version: i.Version,
		})
	}

	return AkuityInstallEntryList
}

func CrossplaneToAkuityAPIClusterCustomization(clusterCustomization *crossplanetypes.ClusterCustomization) (*akuitytypes.ClusterCustomization, error) {
	if clusterCustomization == nil {
		return nil, nil
	}

	kustomization := runtime.RawExtension{}
	if err := yaml.Unmarshal([]byte(clusterCustomization.Kustomization), &kustomization); err != nil {
		return nil, fmt.Errorf("could not unmarshal cluster Kustomization from YAML to runtime raw extension: %w", err)
	}

	return &akuitytypes.ClusterCustomization{
		AutoUpgradeDisabled: clusterCustomization.AutoUpgradeDisabled,
		Kustomization:       kustomization,
		AppReplication:      clusterCustomization.AppReplication,
		RedisTunneling:      clusterCustomization.RedisTunneling,
	}, nil
}

func CrossplaneToAkuityAPIRepoServerDelegate(repoServerDelegate *crossplanetypes.RepoServerDelegate) *akuitytypes.RepoServerDelegate {
	if repoServerDelegate == nil {
		return nil
	}

	return &akuitytypes.RepoServerDelegate{
		ControlPlane: repoServerDelegate.ControlPlane,
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: repoServerDelegate.ManagedCluster.ClusterName,
		},
	}
}

func CrossplaneToAkuityAPICrossplaneExtension(extension *crossplanetypes.CrossplaneExtension) *akuitytypes.CrossplaneExtension {
	if extension == nil {
		return nil
	}

	resources := make([]*akuitytypes.CrossplaneExtensionResource, 0, len(extension.Resources))
	for _, resource := range extension.Resources {
		resources = append(resources, &akuitytypes.CrossplaneExtensionResource{
			Group: resource.Group,
		})
	}
	return &akuitytypes.CrossplaneExtension{Resources: resources}

}

func CrossplaneToAkuityAPIImageUpdaterDelegate(imageUpdaterDelegate *crossplanetypes.ImageUpdaterDelegate) *akuitytypes.ImageUpdaterDelegate {
	if imageUpdaterDelegate == nil {
		return nil
	}

	return &akuitytypes.ImageUpdaterDelegate{
		ControlPlane: imageUpdaterDelegate.ControlPlane,
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: imageUpdaterDelegate.ManagedCluster.ClusterName,
		},
	}
}

func CrossplaneToAkuityAPIAppSetDelegate(appSetDelegate *crossplanetypes.AppSetDelegate) *akuitytypes.AppSetDelegate {
	if appSetDelegate == nil {
		return nil
	}

	return &akuitytypes.AppSetDelegate{
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: appSetDelegate.ManagedCluster.ClusterName,
		},
	}
}

func CrossplaneToAkuityAPIAppsetPolicy(appsetPolicy *crossplanetypes.AppsetPolicy) *akuitytypes.AppsetPolicy {
	if appsetPolicy == nil {
		return nil
	}

	return &akuitytypes.AppsetPolicy{
		Policy:         appsetPolicy.Policy,
		OverridePolicy: appsetPolicy.OverridePolicy,
	}
}

func CrossplaneToAkuityAPIHostAliases(hostAliasesList []*crossplanetypes.HostAliases) []*akuitytypes.HostAliases {
	AkuityHostAliasesList := make([]*akuitytypes.HostAliases, 0)

	for _, h := range hostAliasesList {
		AkuityHostAliasesList = append(AkuityHostAliasesList, &akuitytypes.HostAliases{
			Ip:        h.Ip,
			Hostnames: h.Hostnames,
		})
	}

	return AkuityHostAliasesList
}

func CrossplaneToAkuityAPIAgentPermissionsRules(agentPermissionsRules []*crossplanetypes.AgentPermissionsRule) []*akuitytypes.AgentPermissionsRule {
	if len(agentPermissionsRules) == 0 {
		return nil
	}

	akuityAgentPermissionsRules := make([]*akuitytypes.AgentPermissionsRule, 0, len(agentPermissionsRules))
	for _, a := range agentPermissionsRules {
		copied := a.DeepCopy()
		akuityAgentPermissionsRules = append(akuityAgentPermissionsRules, &akuitytypes.AgentPermissionsRule{
			ApiGroups: copied.ApiGroups,
			Resources: copied.Resources,
			Verbs:     copied.Verbs,
		})
	}
	return akuityAgentPermissionsRules
}

func CrossplaneToAkuityAPIConfigMap(name string, configMapData map[string]string) (*structpb.Struct, error) {
	if len(configMapData) == 0 {
		return nil, nil
	}

	akConfigMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: configMapData,
	}

	akConfigMapPB, err := protobuf.MarshalObjectToProtobufStruct(akConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s configmap to protobuf struct: %w", name, err)
	}

	return akConfigMapPB, nil
}

func CrossplaneToAkuityAPIConfigManagementPlugins(configManagementPlugins map[string]crossplanetypes.ConfigManagementPlugin) ([]*structpb.Struct, error) {
	akConfigManagementPluginsPB := make([]*structpb.Struct, 0)

	for configManagementPluginName, configManagementPlugin := range configManagementPlugins {
		static := make([]*argocdtypes.ParameterAnnouncement, 0)
		for _, pm := range configManagementPlugin.Spec.Parameters.Static {
			static = append(static, &argocdtypes.ParameterAnnouncement{
				Name:           pm.Name,
				Title:          pm.Title,
				Tooltip:        pm.Tooltip,
				Required:       pm.Required,
				ItemType:       pm.ItemType,
				CollectionType: pm.CollectionType,
				String_:        pm.String_,
				Array:          pm.Array,
				Map:            pm.Map,
			})
		}

		akConfigManagementPlugin := argocdtypes.ConfigManagementPlugin{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigManagementPlugin",
				APIVersion: "argocd.akuity.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: configManagementPluginName,
				Annotations: map[string]string{
					argocdtypes.AnnotationCMPEnabled: strconv.FormatBool(configManagementPlugin.Enabled),
					argocdtypes.AnnotationCMPImage:   configManagementPlugin.Image,
				},
			},
			Spec: argocdtypes.PluginSpec{
				Version:  configManagementPlugin.Spec.Version,
				Init:     (*argocdtypes.Command)(configManagementPlugin.Spec.Init),
				Generate: (*argocdtypes.Command)(configManagementPlugin.Spec.Generate),
				Discover: &argocdtypes.Discover{
					Find:     (*argocdtypes.Find)(configManagementPlugin.Spec.Discover.Find),
					FileName: configManagementPlugin.Spec.Discover.FileName,
				},
				Parameters: &argocdtypes.Parameters{
					Static:  static,
					Dynamic: (*argocdtypes.Dynamic)(configManagementPlugin.Spec.Parameters.Dynamic),
				},
				PreserveFileMode: configManagementPlugin.Spec.PreserveFileMode,
			},
		}

		akConfigManagementPluginPB, err := protobuf.MarshalObjectToProtobufStruct(akConfigManagementPlugin)
		if err != nil {
			return nil, fmt.Errorf("could not marshal %s config management plugin to protobuf struct: %w", configManagementPluginName, err)
		}

		akConfigManagementPluginsPB = append(akConfigManagementPluginsPB, akConfigManagementPluginPB)
	}

	return akConfigManagementPluginsPB, nil
}

func CrossplaneToAkuityAPIAkuityIntelligenceExtension(akuityIntelligenceArgoExtension *crossplanetypes.AkuityIntelligenceExtension) *akuitytypes.AkuityIntelligenceExtension {
	if akuityIntelligenceArgoExtension == nil {
		return nil
	}

	return &akuitytypes.AkuityIntelligenceExtension{
		Enabled:                  akuityIntelligenceArgoExtension.Enabled,
		AllowedUsernames:         akuityIntelligenceArgoExtension.AllowedUsernames,
		AllowedGroups:            akuityIntelligenceArgoExtension.AllowedGroups,
		AiSupportEngineerEnabled: akuityIntelligenceArgoExtension.AiSupportEngineerEnabled,
		ModelVersion:             akuityIntelligenceArgoExtension.ModelVersion,
	}
}

func CrossplaneToAkuityAPICustomDeprecatedApis(customDeprecatedApis []*crossplanetypes.CustomDeprecatedAPI) []*akuitytypes.CustomDeprecatedAPI {
	if len(customDeprecatedApis) == 0 {
		return nil
	}

	akuityCustomDeprecatedApis := make([]*akuitytypes.CustomDeprecatedAPI, 0, len(customDeprecatedApis))
	for _, c := range customDeprecatedApis {
		akuityCustomDeprecatedApis = append(akuityCustomDeprecatedApis, &akuitytypes.CustomDeprecatedAPI{
			ApiVersion:                     c.ApiVersion,
			NewApiVersion:                  c.NewApiVersion,
			DeprecatedInKubernetesVersion:  c.DeprecatedInKubernetesVersion,
			UnavailableInKubernetesVersion: c.UnavailableInKubernetesVersion,
		})
	}

	return akuityCustomDeprecatedApis
}

func CrossplaneToAkuityAPIKubeVisionConfig(kubeVisionConfig *crossplanetypes.KubeVisionConfig) *akuitytypes.KubeVisionConfig {
	if kubeVisionConfig == nil {
		return nil
	}

	return &akuitytypes.KubeVisionConfig{
		CveScanConfig: &akuitytypes.CveScanConfig{
			ScanEnabled:    kubeVisionConfig.CveScanConfig.ScanEnabled,
			RescanInterval: kubeVisionConfig.CveScanConfig.RescanInterval,
		},
	}
}

func CrossplaneToAkuityAPIAppInAnyNamespaceConfig(appInAnyNamespaceConfig *crossplanetypes.AppInAnyNamespaceConfig) *akuitytypes.AppInAnyNamespaceConfig {
	if appInAnyNamespaceConfig == nil {
		return nil
	}

	return &akuitytypes.AppInAnyNamespaceConfig{
		Enabled: appInAnyNamespaceConfig.Enabled,
	}
}

func CrossplaneToAkuityAPIAppsetPlugins(appsetPlugins []*crossplanetypes.AppsetPlugins) []*akuitytypes.AppsetPlugins {
	if len(appsetPlugins) == 0 {
		return nil
	}

	akuityAppsetPlugins := make([]*akuitytypes.AppsetPlugins, 0, len(appsetPlugins))
	for _, p := range appsetPlugins {
		akuityAppsetPlugins = append(akuityAppsetPlugins, &akuitytypes.AppsetPlugins{
			Name:           p.Name,
			Token:          p.Token,
			BaseUrl:        p.BaseUrl,
			RequestTimeout: p.RequestTimeout,
		})
	}

	return akuityAppsetPlugins
}

func CrossplaneToAkuityAPIApplicationSetExtension(applicationSetExtension *crossplanetypes.ApplicationSetExtension) *akuitytypes.ApplicationSetExtension {
	if applicationSetExtension == nil {
		return nil
	}

	return &akuitytypes.ApplicationSetExtension{
		Enabled: applicationSetExtension.Enabled,
	}
}

func CrossplaneToAkuityAPIAppReconciliationsRateLimiting(appReconciliationsRateLimiting *crossplanetypes.AppReconciliationsRateLimiting) (*akuitytypes.AppReconciliationsRateLimiting, error) {
	if appReconciliationsRateLimiting == nil {
		return nil, nil
	}

	rl := &akuitytypes.AppReconciliationsRateLimiting{}

	if appReconciliationsRateLimiting.BucketRateLimiting != nil {
		bucket := appReconciliationsRateLimiting.BucketRateLimiting
		rl.BucketRateLimiting = &akuitytypes.BucketRateLimiting{
			Enabled:    bucket.Enabled,
			BucketSize: bucket.BucketSize,
			BucketQps:  bucket.BucketQps,
		}
	}

	if appReconciliationsRateLimiting.ItemRateLimiting != nil {
		item := appReconciliationsRateLimiting.ItemRateLimiting
		rl.ItemRateLimiting = &akuitytypes.ItemRateLimiting{
			Enabled:         item.Enabled,
			FailureCooldown: item.FailureCooldown,
			BaseDelay:       item.BaseDelay,
			MaxDelay:        item.MaxDelay,
		}

		if item.BackoffFactorString != "" {
			backoff, err := strconv.ParseFloat(item.BackoffFactorString, 32)
			if err != nil {
				return nil, fmt.Errorf("could not parse backoff factor %q as float: %w", item.BackoffFactorString, err)
			}
			rl.ItemRateLimiting.BackoffFactor = float32(backoff)
		}
	}

	return rl, nil
}
