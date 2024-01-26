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
type InstanceSpec struct {
	IpAllowList                  []*IPAllowListEntry            `json:"ipAllowList,omitempty"`
	Subdomain                    string                         `json:"subdomain,omitempty"`
	DeclarativeManagementEnabled bool                           `json:"declarativeManagementEnabled,omitempty"`
	Extensions                   []*ArgoCDExtensionInstallEntry `json:"extensions,omitempty"`
	ClusterCustomizationDefaults *ClusterCustomization          `json:"clusterCustomizationDefaults,omitempty"`
	ImageUpdaterEnabled          bool                           `json:"imageUpdaterEnabled,omitempty"`
	BackendIpAllowListEnabled    bool                           `json:"backendIpAllowListEnabled,omitempty"`
	RepoServerDelegate           *RepoServerDelegate            `json:"repoServerDelegate,omitempty"`
	AuditExtensionEnabled        bool                           `json:"auditExtensionEnabled,omitempty"`
	SyncHistoryExtensionEnabled  bool                           `json:"syncHistoryExtensionEnabled,omitempty"`
	ImageUpdaterDelegate         *ImageUpdaterDelegate          `json:"imageUpdaterDelegate,omitempty"`
	AppSetDelegate               *AppSetDelegate                `json:"appSetDelegate,omitempty"`
	AssistantExtensionEnabled    bool                           `json:"assistantExtensionEnabled,omitempty"`
	AppsetPolicy                 *AppsetPolicy                  `json:"appsetPolicy,omitempty"`
	HostAliases                  []*HostAliases                 `json:"hostAliases,omitempty"`
	AgentPermissionsRules        []*AgentPermissionsRule        `json:"agentPermissionsRules,omitempty"`
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
