// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2026 Akuity, Inc.
*/

package v1alpha1

type KargoAgentSize string

// +kubebuilder:object:generate=true
type KargoAgent struct {
	Spec KargoAgentSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoAgentSpec struct {
	Description string         `json:"description,omitempty"`
	Data        KargoAgentData `json:"data,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoResources struct {
	Mem string `json:"mem,omitempty"`
	Cpu string `json:"cpu,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoControllerAutoScalingConfig struct {
	ResourceMinimum *KargoResources `json:"resourceMinimum,omitempty"`
	ResourceMaximum *KargoResources `json:"resourceMaximum,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoAutoscalerConfig struct {
	KargoController *KargoControllerAutoScalingConfig `json:"kargoController,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoAgentCustomAgentSizeConfig struct {
	KargoController *KargoResources `json:"kargoController,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoAgentData struct {
	Size                  KargoAgentSize                   `json:"size,omitempty"`
	AutoUpgradeDisabled   *bool                            `json:"autoUpgradeDisabled,omitempty"`
	TargetVersion         string                           `json:"targetVersion,omitempty"`
	Kustomization         string                           `json:"kustomization,omitempty"`
	RemoteArgocd          string                           `json:"remoteArgocd,omitempty"`
	AkuityManaged         *bool                            `json:"akuityManaged,omitempty"`
	ArgocdNamespace       string                           `json:"argocdNamespace,omitempty"`
	SelfManagedArgocdUrl  string                           `json:"selfManagedArgocdUrl,omitempty"`
	AllowedJobSa          []string                         `json:"allowedJobSa,omitempty"`
	MaintenanceMode       *bool                            `json:"maintenanceMode,omitempty"`
	MaintenanceModeExpiry *string                          `json:"maintenanceModeExpiry,omitempty"`
	PodInheritMetadata    *bool                            `json:"podInheritMetadata,omitempty"`
	AutoscalerConfig      *KargoAutoscalerConfig           `json:"autoscalerConfig,omitempty"`
	CustomAgentSizeConfig *KargoAgentCustomAgentSizeConfig `json:"customAgentSizeConfig,omitempty"`
}
