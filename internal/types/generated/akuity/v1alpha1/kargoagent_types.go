// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2025 Akuity, Inc.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KargoAgent is the Schema for the KargoAgent API
type KargoAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KargoAgentSpec `json:"spec,omitempty"`
}

type KargoAgentSize string

//+kubebuilder:object:root=true

// KargoAgentList contains a list of KargoAgent
type KargoAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KargoAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KargoAgent{}, &KargoAgentList{})
}

type KargoAgentSpec struct {
	Description string         `json:"description,omitempty"`
	Data        KargoAgentData `json:"data,omitempty"`
}

type KargoAgentData struct {
	Size                 KargoAgentSize       `json:"size,omitempty"`
	AutoUpgradeDisabled  *bool                `json:"autoUpgradeDisabled,omitempty"`
	TargetVersion        string               `json:"targetVersion,omitempty"`
	Kustomization        runtime.RawExtension `json:"kustomization,omitempty"`
	RemoteArgocd         string               `json:"remoteArgocd,omitempty"`
	AkuityManaged        bool                 `json:"akuityManaged,omitempty"`
	ArgocdNamespace      string               `json:"argocdNamespace,omitempty"`
	SelfManagedArgocdUrl string               `json:"selfManagedArgocdUrl,omitempty"`
}
