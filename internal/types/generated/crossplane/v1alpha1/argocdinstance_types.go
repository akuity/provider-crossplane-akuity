// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2023 Akuity, Inc.
*/

package v1alpha1

// +kubebuilder:object:generate=true
type ArgoCD struct {
	Spec ArgoCDSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:generate=true
type ArgoCDList struct {
	Items []ArgoCD `json:"items"`
}

// +kubebuilder:object:generate=true
type ArgoCDSpec struct {
	Description  string       `json:"description,omitempty"`
	Version      string       `json:"version"`
	InstanceSpec InstanceSpec `json:"instanceSpec,omitempty"`
}

// +kubebuilder:object:generate=true
type ArgoCDExtensionInstallEntry struct {
	Id      string `json:"id,omitempty"`
	Version string `json:"version,omitempty"`
}

// +kubebuilder:object:generate=true
type ClusterCustomization struct {
	AutoUpgradeDisabled bool   `json:"autoUpgradeDisabled,omitempty"`
	Kustomization       string `json:"kustomization,omitempty"`
	AppReplication      bool   `json:"appReplication,omitempty"`
	RedisTunneling      bool   `json:"redisTunneling,omitempty"`
}

// +kubebuilder:object:generate=true
type AppsetPolicy struct {
	Policy         string `json:"policy,omitempty"`
	OverridePolicy bool   `json:"overridePolicy,omitempty"`
}

// +kubebuilder:object:generate=true
type AgentPermissionsRule struct {
	ApiGroups []string `json:"apiGroups,omitempty"`
	Resources []string `json:"resources,omitempty"`
	Verbs     []string `json:"verbs,omitempty"`
}

// +kubebuilder:object:generate=true
type CrossplaneExtensionResource struct {
	Group string `json:"group,omitempty"`
}

// +kubebuilder:object:generate=true
type CrossplaneExtension struct {
	Resources []*CrossplaneExtensionResource `json:"resources,omitempty"`
}

// +kubebuilder:object:generate=true
type KubeVisionArgoExtension struct {
	Enabled          bool     `json:"enabled,omitempty"`
	AllowedUsernames []string `json:"allowedUsernames,omitempty"`
	AllowedGroups    []string `json:"allowedGroups,omitempty"`
}

// +kubebuilder:object:generate=true
type KubeVisionConfig struct {
	CveScanConfig *CveScanConfig `json:"cveScanConfig,omitempty"`
}

// +kubebuilder:object:generate=true
type AppInAnyNamespaceConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

// +kubebuilder:object:generate=true
type CustomDeprecatedAPI struct {
	ApiVersion                     string `json:"apiVersion,omitempty"`
	NewApiVersion                  string `json:"newApiVersion,omitempty"`
	DeprecatedInKubernetesVersion  string `json:"deprecatedInKubernetesVersion,omitempty"`
	UnavailableInKubernetesVersion string `json:"unavailableInKubernetesVersion,omitempty"`
}

// +kubebuilder:object:generate=true
type CveScanConfig struct {
	ScanEnabled    bool   `json:"scanEnabled,omitempty"`
	RescanInterval string `json:"rescanInterval,omitempty"`
}

// +kubebuilder:object:generate=true
type ObjectSelector struct {
	MatchLabels      map[string]string           `json:"matchLabels,omitempty"`
	MatchExpressions []*LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// +kubebuilder:object:generate=true
type LabelSelectorRequirement struct {
	Key      *string  `json:"key,omitempty"`
	Operator *string  `json:"operator,omitempty"`
	Values   []string `json:"values,omitempty"`
}

// +kubebuilder:object:generate=true
type ClusterSecretMapping struct {
	Clusters *ObjectSelector `json:"clusters,omitempty"`
	Secrets  *ObjectSelector `json:"secrets,omitempty"`
}

// +kubebuilder:object:generate=true
type SecretsManagementConfig struct {
	Sources      []*ClusterSecretMapping `json:"sources,omitempty"`
	Destinations []*ClusterSecretMapping `json:"destinations,omitempty"`
}

// +kubebuilder:object:generate=true
type AISupportEngineerExtension struct {
	Enabled bool `json:"enabled,omitempty"`
}

// +kubebuilder:object:generate=true
type InstanceSpec struct {
	IpAllowList                     []*IPAllowListEntry            `json:"ipAllowList,omitempty"`
	Subdomain                       string                         `json:"subdomain,omitempty"`
	DeclarativeManagementEnabled    bool                           `json:"declarativeManagementEnabled,omitempty"`
	Extensions                      []*ArgoCDExtensionInstallEntry `json:"extensions,omitempty"`
	ClusterCustomizationDefaults    *ClusterCustomization          `json:"clusterCustomizationDefaults,omitempty"`
	ImageUpdaterEnabled             bool                           `json:"imageUpdaterEnabled,omitempty"`
	BackendIpAllowListEnabled       bool                           `json:"backendIpAllowListEnabled,omitempty"`
	RepoServerDelegate              *RepoServerDelegate            `json:"repoServerDelegate,omitempty"`
	AuditExtensionEnabled           bool                           `json:"auditExtensionEnabled,omitempty"`
	SyncHistoryExtensionEnabled     bool                           `json:"syncHistoryExtensionEnabled,omitempty"`
	CrossplaneExtension             *CrossplaneExtension           `json:"crossplaneExtension,omitempty"`
	ImageUpdaterDelegate            *ImageUpdaterDelegate          `json:"imageUpdaterDelegate,omitempty"`
	AppSetDelegate                  *AppSetDelegate                `json:"appSetDelegate,omitempty"`
	AssistantExtensionEnabled       bool                           `json:"assistantExtensionEnabled,omitempty"`
	AppsetPolicy                    *AppsetPolicy                  `json:"appsetPolicy,omitempty"`
	HostAliases                     []*HostAliases                 `json:"hostAliases,omitempty"`
	AgentPermissionsRules           []*AgentPermissionsRule        `json:"agentPermissionsRules,omitempty"`
	Fqdn                            string                         `json:"fqdn,omitempty"`
	MultiClusterK8SDashboardEnabled bool                           `json:"multiClusterK8sDashboardEnabled,omitempty"`
	KubeVisionArgoExtension         *KubeVisionArgoExtension       `json:"kubeVisionArgoExtension,omitempty"`
	ImageUpdaterVersion             string                         `json:"imageUpdaterVersion,omitempty"`
	CustomDeprecatedApis            []*CustomDeprecatedAPI         `json:"customDeprecatedApis,omitempty"`
	KubeVisionConfig                *KubeVisionConfig              `json:"kubeVisionConfig,omitempty"`
	AppInAnyNamespaceConfig         *AppInAnyNamespaceConfig       `json:"appInAnyNamespaceConfig,omitempty"`
	Basepath                        string                         `json:"basepath,omitempty"`
	AppsetProgressiveSyncsEnabled   bool                           `json:"appsetProgressiveSyncsEnabled,omitempty"`
	AiSupportEngineerExtension      *AISupportEngineerExtension    `json:"aiSupportEngineerExtension,omitempty"`
	Secrets                         *SecretsManagementConfig       `json:"secrets,omitempty"`
}

// +kubebuilder:object:generate=true
type ManagedCluster struct {
	ClusterName string `json:"clusterName,omitempty"`
}

// +kubebuilder:object:generate=true
type RepoServerDelegate struct {
	ControlPlane   bool            `json:"controlPlane,omitempty"`
	ManagedCluster *ManagedCluster `json:"managedCluster,omitempty"`
}

// +kubebuilder:object:generate=true
type ImageUpdaterDelegate struct {
	ControlPlane   bool            `json:"controlPlane,omitempty"`
	ManagedCluster *ManagedCluster `json:"managedCluster,omitempty"`
}

// +kubebuilder:object:generate=true
type AppSetDelegate struct {
	ManagedCluster *ManagedCluster `json:"managedCluster,omitempty"`
}

// +kubebuilder:object:generate=true
type IPAllowListEntry struct {
	Ip          string `json:"ip,omitempty"`
	Description string `json:"description,omitempty"`
}

// +kubebuilder:object:generate=true
type HostAliases struct {
	Ip        string   `json:"ip,omitempty"`
	Hostnames []string `json:"hostnames,omitempty"`
}
