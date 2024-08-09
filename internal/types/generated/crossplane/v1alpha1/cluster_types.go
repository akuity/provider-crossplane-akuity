// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2023 Akuity, Inc.
*/

package v1alpha1

type ClusterSize string

type DirectClusterType string

// +kubebuilder:object:generate=true
type Cluster struct {
	Spec ClusterSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:generate=true
type ClusterList struct {
	Items []Cluster `json:"items"`
}

// +kubebuilder:object:generate=true
type ClusterSpec struct {
	Description     string      `json:"description,omitempty"`
	NamespaceScoped bool        `json:"namespaceScoped,omitempty"`
	Data            ClusterData `json:"data,omitempty"`
}

// +kubebuilder:object:generate=true
type DirectClusterSpec struct {
	ClusterType     DirectClusterType `json:"clusterType,omitempty"`
	KargoInstanceId *string           `json:"kargoInstanceId,omitempty"`
	Server          *string           `json:"server,omitempty"`
	Organization    *string           `json:"organization,omitempty"`
	Token           *string           `json:"token,omitempty"`
	CaData          *string           `json:"caData,omitempty"`
}

// +kubebuilder:object:generate=true
type ManagedClusterConfig struct {
	SecretName string `json:"secretName,omitempty"`
	SecretKey  string `json:"secretKey,omitempty"`
}

// +kubebuilder:object:generate=true
type ClusterData struct {
	Size                            ClusterSize           `json:"size,omitempty"`
	AutoUpgradeDisabled             bool                  `json:"autoUpgradeDisabled,omitempty"`
	Kustomization                   string                `json:"kustomization,omitempty"`
	AppReplication                  bool                  `json:"appReplication,omitempty"`
	TargetVersion                   string                `json:"targetVersion,omitempty"`
	RedisTunneling                  bool                  `json:"redisTunneling,omitempty"`
	DirectClusterSpec               *DirectClusterSpec    `json:"directClusterSpec,omitempty"`
	DatadogAnnotationsEnabled       *bool                 `json:"datadogAnnotationsEnabled,omitempty"`
	EksAddonEnabled                 *bool                 `json:"eksAddonEnabled,omitempty"`
	ManagedClusterConfig            *ManagedClusterConfig `json:"managedClusterConfig,omitempty"`
	MaintenanceMode                 *bool                 `json:"maintenanceMode,omitempty"`
	MultiClusterK8SDashboardEnabled *bool                 `json:"multiClusterK8sDashboardEnabled,omitempty"`
}
