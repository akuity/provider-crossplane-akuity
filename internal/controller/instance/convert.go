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

package instance

import (
	"fmt"
	"strconv"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	argocdtypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/argocd/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
)

// BuildApplyInstanceRequest assembles the ApplyInstanceRequest proto
// from the managed Instance's forProvider spec. OrganizationId is left
// empty and filled by the client's ApplyInstance method so this
// function stays pure and testable. Exported so tests can round-trip
// through the same builder the controller uses.
func BuildApplyInstanceRequest(instance v1alpha1.Instance) (*argocdv1.ApplyInstanceRequest, error) {
	argocdPB, err := specToArgoCDPB(instance.Spec.ForProvider.Name, instance.Spec.ForProvider.ArgoCD)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd spec to protobuf: %w", err)
	}

	argocdConfigMapPB, err := specToConfigMapPB(observation.ArgocdCMKey, instance.Spec.ForProvider.ArgoCDConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd configmap to protobuf: %w", err)
	}

	argocdRbacConfigMapPB, err := specToConfigMapPB(observation.ArgocdRBACCMKey, instance.Spec.ForProvider.ArgoCDRBACConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd rbac configmap to protobuf: %w", err)
	}

	argocdNotificationsConfigMapPB, err := specToConfigMapPB(observation.ArgocdNotificationsCMKey, instance.Spec.ForProvider.ArgoCDNotificationsConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd notifications configmap to protobuf: %w", err)
	}

	argocdImageUpdaterConfigMapPB, err := specToConfigMapPB(observation.ArgocdImageUpdaterCMKey, instance.Spec.ForProvider.ArgoCDImageUpdaterConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd image updater configmap to protobuf: %w", err)
	}

	argocdImageUpdaterSshConfigMapPB, err := specToConfigMapPB(observation.ArgocdImageUpdaterSSHCMKey, instance.Spec.ForProvider.ArgoCDImageUpdaterSSHConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd image updater ssh configmap to protobuf: %w", err)
	}

	argocdKnownHostsConfigMapPB, err := specToConfigMapPB(observation.ArgocdSSHKnownHostsCMKey, instance.Spec.ForProvider.ArgoCDSSHKnownHostsConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd known hosts configmap to protobuf: %w", err)
	}

	argocdTlsCertsConfigMapPB, err := specToConfigMapPB(observation.ArgocdTLSCertsCMKey, instance.Spec.ForProvider.ArgoCDTLSCertsConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd tls certs configmap to protobuf: %w", err)
	}

	configManagementPluginsPB, err := specToConfigManagementPluginsPB(instance.Spec.ForProvider.ConfigManagementPlugins)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd config management plugins to protobuf: %w", err)
	}

	return &argocdv1.ApplyInstanceRequest{
		IdType:                    idv1.Type_NAME,
		Id:                        instance.Spec.ForProvider.Name,
		Argocd:                    argocdPB,
		ArgocdConfigmap:           argocdConfigMapPB,
		ArgocdRbacConfigmap:       argocdRbacConfigMapPB,
		NotificationsConfigmap:    argocdNotificationsConfigMapPB,
		ImageUpdaterConfigmap:     argocdImageUpdaterConfigMapPB,
		ImageUpdaterSshConfigmap:  argocdImageUpdaterSshConfigMapPB,
		ArgocdKnownHostsConfigmap: argocdKnownHostsConfigMapPB,
		ArgocdTlsCertsConfigmap:   argocdTlsCertsConfigMapPB,
		ConfigManagementPlugins:   configManagementPluginsPB,
	}, nil
}

func specToArgoCDPB(name string, instance *crossplanetypes.ArgoCD) (*structpb.Struct, error) {
	instanceSpec, err := SpecToInstanceSpec(instance.Spec.InstanceSpec)
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

	argocdPB, err := marshal.APIModelToPBStruct(argocd)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd spec to protobuf: %w", err)
	}

	return argocdPB, nil
}

func SpecToInstanceSpec(instanceSpec crossplanetypes.InstanceSpec) (akuitytypes.InstanceSpec, error) {
	clusterCustomization, err := specToClusterCustomization(instanceSpec.ClusterCustomizationDefaults)
	if err != nil {
		return akuitytypes.InstanceSpec{}, fmt.Errorf("could not build instance argocd instance spec: %w", err)
	}

	appReconciliationsRateLimiting, err := specToAppReconciliationsRateLimiting(instanceSpec.AppReconciliationsRateLimiting)
	if err != nil {
		return akuitytypes.InstanceSpec{}, fmt.Errorf("could not build instance app reconciliations rate limiting config: %w", err)
	}

	return akuitytypes.InstanceSpec{
		IpAllowList:                     specToIPAllowList(instanceSpec.IpAllowList),
		Subdomain:                       instanceSpec.Subdomain,
		DeclarativeManagementEnabled:    instanceSpec.DeclarativeManagementEnabled,
		Extensions:                      specToExtensionInstallEntries(instanceSpec.Extensions),
		ClusterCustomizationDefaults:    clusterCustomization,
		ImageUpdaterEnabled:             instanceSpec.ImageUpdaterEnabled,
		BackendIpAllowListEnabled:       instanceSpec.BackendIpAllowListEnabled,
		RepoServerDelegate:              specToRepoServerDelegate(instanceSpec.RepoServerDelegate),
		AuditExtensionEnabled:           instanceSpec.AuditExtensionEnabled,
		SyncHistoryExtensionEnabled:     instanceSpec.SyncHistoryExtensionEnabled,
		CrossplaneExtension:             specToCrossplaneExtension(instanceSpec.CrossplaneExtension),
		ImageUpdaterDelegate:            specToImageUpdaterDelegate(instanceSpec.ImageUpdaterDelegate),
		AppSetDelegate:                  specToAppSetDelegate(instanceSpec.AppSetDelegate),
		AssistantExtensionEnabled:       instanceSpec.AssistantExtensionEnabled,
		AppsetPolicy:                    specToAppsetPolicy(instanceSpec.AppsetPolicy),
		HostAliases:                     specToHostAliases(instanceSpec.HostAliases),
		AgentPermissionsRules:           specToAgentPermissionsRules(instanceSpec.AgentPermissionsRules),
		Fqdn:                            instanceSpec.Fqdn,
		MultiClusterK8SDashboardEnabled: instanceSpec.MultiClusterK8SDashboardEnabled,
		AkuityIntelligenceExtension:     specToAkuityIntelligenceExtension(instanceSpec.AkuityIntelligenceExtension),
		ImageUpdaterVersion:             instanceSpec.ImageUpdaterVersion,
		CustomDeprecatedApis:            specToCustomDeprecatedApis(instanceSpec.CustomDeprecatedApis),
		KubeVisionConfig:                specToKubeVisionConfig(instanceSpec.KubeVisionConfig),
		AppInAnyNamespaceConfig:         specToAppInAnyNamespaceConfig(instanceSpec.AppInAnyNamespaceConfig),
		Basepath:                        instanceSpec.Basepath,
		AppsetProgressiveSyncsEnabled:   instanceSpec.AppsetProgressiveSyncsEnabled,
		AppsetPlugins:                   specToAppsetPlugins(instanceSpec.AppsetPlugins),
		ApplicationSetExtension:         specToApplicationSetExtension(instanceSpec.ApplicationSetExtension),
		AppReconciliationsRateLimiting:  appReconciliationsRateLimiting,
	}, nil
}

func specToIPAllowList(ipAllowList []*crossplanetypes.IPAllowListEntry) []*akuitytypes.IPAllowListEntry {
	out := make([]*akuitytypes.IPAllowListEntry, 0, len(ipAllowList))
	for _, i := range ipAllowList {
		out = append(out, &akuitytypes.IPAllowListEntry{
			Ip:          i.Ip,
			Description: i.Description,
		})
	}
	return out
}

func specToExtensionInstallEntries(list []*crossplanetypes.ArgoCDExtensionInstallEntry) []*akuitytypes.ArgoCDExtensionInstallEntry {
	out := make([]*akuitytypes.ArgoCDExtensionInstallEntry, 0, len(list))
	for _, i := range list {
		out = append(out, &akuitytypes.ArgoCDExtensionInstallEntry{
			Id:      i.Id,
			Version: i.Version,
		})
	}
	return out
}

func specToClusterCustomization(in *crossplanetypes.ClusterCustomization) (*akuitytypes.ClusterCustomization, error) {
	if in == nil {
		return nil, nil
	}
	kustomization := runtime.RawExtension{}
	if err := yaml.Unmarshal([]byte(in.Kustomization), &kustomization); err != nil {
		return nil, fmt.Errorf("could not unmarshal cluster Kustomization from YAML to runtime raw extension: %w", err)
	}
	return &akuitytypes.ClusterCustomization{
		AutoUpgradeDisabled: in.AutoUpgradeDisabled,
		Kustomization:       kustomization,
		AppReplication:      in.AppReplication,
		RedisTunneling:      in.RedisTunneling,
	}, nil
}

func specToRepoServerDelegate(in *crossplanetypes.RepoServerDelegate) *akuitytypes.RepoServerDelegate {
	if in == nil {
		return nil
	}
	return &akuitytypes.RepoServerDelegate{
		ControlPlane: in.ControlPlane,
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: in.ManagedCluster.ClusterName,
		},
	}
}

func specToCrossplaneExtension(in *crossplanetypes.CrossplaneExtension) *akuitytypes.CrossplaneExtension {
	if in == nil {
		return nil
	}
	resources := make([]*akuitytypes.CrossplaneExtensionResource, 0, len(in.Resources))
	for _, r := range in.Resources {
		resources = append(resources, &akuitytypes.CrossplaneExtensionResource{Group: r.Group})
	}
	return &akuitytypes.CrossplaneExtension{Resources: resources}
}

func specToImageUpdaterDelegate(in *crossplanetypes.ImageUpdaterDelegate) *akuitytypes.ImageUpdaterDelegate {
	if in == nil {
		return nil
	}
	return &akuitytypes.ImageUpdaterDelegate{
		ControlPlane: in.ControlPlane,
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: in.ManagedCluster.ClusterName,
		},
	}
}

func specToAppSetDelegate(in *crossplanetypes.AppSetDelegate) *akuitytypes.AppSetDelegate {
	if in == nil {
		return nil
	}
	return &akuitytypes.AppSetDelegate{
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: in.ManagedCluster.ClusterName,
		},
	}
}

func specToAppsetPolicy(in *crossplanetypes.AppsetPolicy) *akuitytypes.AppsetPolicy {
	if in == nil {
		return nil
	}
	return &akuitytypes.AppsetPolicy{
		Policy:         in.Policy,
		OverridePolicy: in.OverridePolicy,
	}
}

func specToHostAliases(list []*crossplanetypes.HostAliases) []*akuitytypes.HostAliases {
	out := make([]*akuitytypes.HostAliases, 0, len(list))
	for _, h := range list {
		out = append(out, &akuitytypes.HostAliases{
			Ip:        h.Ip,
			Hostnames: h.Hostnames,
		})
	}
	return out
}

func specToAgentPermissionsRules(rules []*crossplanetypes.AgentPermissionsRule) []*akuitytypes.AgentPermissionsRule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]*akuitytypes.AgentPermissionsRule, 0, len(rules))
	for _, r := range rules {
		copied := r.DeepCopy()
		out = append(out, &akuitytypes.AgentPermissionsRule{
			ApiGroups: copied.ApiGroups,
			Resources: copied.Resources,
			Verbs:     copied.Verbs,
		})
	}
	return out
}

func specToConfigMapPB(name string, data map[string]string) (*structpb.Struct, error) {
	if len(data) == 0 {
		return nil, nil
	}
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       data,
	}
	pb, err := marshal.APIModelToPBStruct(cm)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s configmap to protobuf struct: %w", name, err)
	}
	return pb, nil
}

func specToConfigManagementPluginsPB(plugins map[string]crossplanetypes.ConfigManagementPlugin) ([]*structpb.Struct, error) {
	out := make([]*structpb.Struct, 0)

	for name, plugin := range plugins {
		static := make([]*argocdtypes.ParameterAnnouncement, 0)
		for _, pm := range plugin.Spec.Parameters.Static {
			static = append(static, &argocdtypes.ParameterAnnouncement{
				Name:           pm.Name,
				Title:          pm.Title,
				Tooltip:        pm.Tooltip,
				Required:       ptr.To(pm.Required),
				ItemType:       pm.ItemType,
				CollectionType: pm.CollectionType,
				String_:        pm.String_,
				Array:          pm.Array,
				Map:            pm.Map,
			})
		}

		cmp := argocdtypes.ConfigManagementPlugin{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigManagementPlugin",
				APIVersion: "argocd.akuity.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					argocdtypes.AnnotationCMPEnabled: strconv.FormatBool(plugin.Enabled),
					argocdtypes.AnnotationCMPImage:   plugin.Image,
				},
			},
			Spec: argocdtypes.PluginSpec{
				Version:  plugin.Spec.Version,
				Init:     (*argocdtypes.Command)(plugin.Spec.Init),
				Generate: (*argocdtypes.Command)(plugin.Spec.Generate),
				Discover: &argocdtypes.Discover{
					Find:     (*argocdtypes.Find)(plugin.Spec.Discover.Find),
					FileName: plugin.Spec.Discover.FileName,
				},
				Parameters: &argocdtypes.Parameters{
					Static:  static,
					Dynamic: (*argocdtypes.Dynamic)(plugin.Spec.Parameters.Dynamic),
				},
				PreserveFileMode: ptr.To(plugin.Spec.PreserveFileMode),
			},
		}

		pb, err := marshal.APIModelToPBStruct(cmp)
		if err != nil {
			return nil, fmt.Errorf("could not marshal %s config management plugin to protobuf struct: %w", name, err)
		}
		out = append(out, pb)
	}

	return out, nil
}

func specToAkuityIntelligenceExtension(in *crossplanetypes.AkuityIntelligenceExtension) *akuitytypes.AkuityIntelligenceExtension {
	if in == nil {
		return nil
	}
	return &akuitytypes.AkuityIntelligenceExtension{
		Enabled:                  in.Enabled,
		AllowedUsernames:         in.AllowedUsernames,
		AllowedGroups:            in.AllowedGroups,
		AiSupportEngineerEnabled: in.AiSupportEngineerEnabled,
		ModelVersion:             in.ModelVersion,
	}
}

func specToCustomDeprecatedApis(list []*crossplanetypes.CustomDeprecatedAPI) []*akuitytypes.CustomDeprecatedAPI {
	if len(list) == 0 {
		return nil
	}
	out := make([]*akuitytypes.CustomDeprecatedAPI, 0, len(list))
	for _, c := range list {
		out = append(out, &akuitytypes.CustomDeprecatedAPI{
			ApiVersion:                     c.ApiVersion,
			NewApiVersion:                  c.NewApiVersion,
			DeprecatedInKubernetesVersion:  c.DeprecatedInKubernetesVersion,
			UnavailableInKubernetesVersion: c.UnavailableInKubernetesVersion,
		})
	}
	return out
}

func specToKubeVisionConfig(in *crossplanetypes.KubeVisionConfig) *akuitytypes.KubeVisionConfig {
	if in == nil {
		return nil
	}
	return &akuitytypes.KubeVisionConfig{
		CveScanConfig: &akuitytypes.CveScanConfig{
			ScanEnabled:    in.CveScanConfig.ScanEnabled,
			RescanInterval: in.CveScanConfig.RescanInterval,
		},
	}
}

func specToAppInAnyNamespaceConfig(in *crossplanetypes.AppInAnyNamespaceConfig) *akuitytypes.AppInAnyNamespaceConfig {
	if in == nil {
		return nil
	}
	return &akuitytypes.AppInAnyNamespaceConfig{
		Enabled: in.Enabled,
	}
}

func specToAppsetPlugins(list []*crossplanetypes.AppsetPlugins) []*akuitytypes.AppsetPlugins {
	if len(list) == 0 {
		return nil
	}
	out := make([]*akuitytypes.AppsetPlugins, 0, len(list))
	for _, p := range list {
		out = append(out, &akuitytypes.AppsetPlugins{
			Name:           p.Name,
			Token:          p.Token,
			BaseUrl:        p.BaseUrl,
			RequestTimeout: p.RequestTimeout,
		})
	}
	return out
}

func specToApplicationSetExtension(in *crossplanetypes.ApplicationSetExtension) *akuitytypes.ApplicationSetExtension {
	if in == nil {
		return nil
	}
	return &akuitytypes.ApplicationSetExtension{
		Enabled: in.Enabled,
	}
}

func specToAppReconciliationsRateLimiting(in *crossplanetypes.AppReconciliationsRateLimiting) (*akuitytypes.AppReconciliationsRateLimiting, error) {
	if in == nil {
		return nil, nil
	}
	rl := &akuitytypes.AppReconciliationsRateLimiting{}

	if in.BucketRateLimiting != nil {
		bucket := in.BucketRateLimiting
		rl.BucketRateLimiting = &akuitytypes.BucketRateLimiting{
			Enabled:    bucket.Enabled,
			BucketSize: bucket.BucketSize,
			BucketQps:  bucket.BucketQps,
		}
	}

	if in.ItemRateLimiting != nil {
		item := in.ItemRateLimiting
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
