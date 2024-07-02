// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2023 Akuity, Inc.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Cluster is the Schema for the cluster API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSpec `json:"spec,omitempty"`
}

type ClusterSize string

type DirectClusterType string

//+kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}

type ClusterSpec struct {
	Description     string      `json:"description,omitempty"`
	NamespaceScoped bool        `json:"namespaceScoped,omitempty"`
	Data            ClusterData `json:"data,omitempty"`
}

type DirectClusterSpec struct {
	ClusterType     DirectClusterType `json:"clusterType,omitempty"`
	KargoInstanceId *string           `json:"kargoInstanceId,omitempty"`
	Server          *string           `json:"server,omitempty"`
	Organization    *string           `json:"organization,omitempty"`
	Token           *string           `json:"token,omitempty"`
	CaData          *string           `json:"caData,omitempty"`
}

type ManagedClusterConfig struct {
	SecretName string `json:"secretName,omitempty"`
	SecretKey  string `json:"secretKey,omitempty"`
}

type ClusterData struct {
	Size                      ClusterSize           `json:"size,omitempty"`
	AutoUpgradeDisabled       *bool                 `json:"autoUpgradeDisabled,omitempty"`
	Kustomization             runtime.RawExtension  `json:"kustomization,omitempty"`
	AppReplication            *bool                 `json:"appReplication,omitempty"`
	TargetVersion             string                `json:"targetVersion,omitempty"`
	RedisTunneling            *bool                 `json:"redisTunneling,omitempty"`
	DirectClusterSpec         *DirectClusterSpec    `json:"directClusterSpec,omitempty"`
	DatadogAnnotationsEnabled *bool                 `json:"datadogAnnotationsEnabled,omitempty"`
	EksAddonEnabled           *bool                 `json:"eksAddonEnabled,omitempty"`
	ManagedClusterConfig      *ManagedClusterConfig `json:"managedClusterConfig,omitempty"`
	MaintenanceMode           *bool                 `json:"maintenanceMode,omitempty"`
}
