// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2023 Akuity, Inc.
*/

package v1alpha1

type ClusterSize string

// +kubebuilder:object:generate=true
type Cluster struct {
	Spec ClusterSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:generate=true
type ClusterSpec struct {
	Description     string      `json:"description,omitempty"`
	NamespaceScoped bool        `json:"namespaceScoped,omitempty"`
	Data            ClusterData `json:"data,omitempty"`
}

// +kubebuilder:object:generate=true
type ClusterData struct {
	Size                ClusterSize `json:"size,omitempty"`
	AutoUpgradeDisabled bool        `json:"autoUpgradeDisabled,omitempty"`
	Kustomization       string      `json:"kustomization,omitempty"`
	AppReplication      bool        `json:"appReplication,omitempty"`
	TargetVersion       string      `json:"targetVersion,omitempty"`
	RedisTunneling      bool        `json:"redisTunneling,omitempty"`
}
