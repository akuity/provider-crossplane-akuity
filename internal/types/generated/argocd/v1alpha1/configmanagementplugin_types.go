// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2023 Akuity, Inc.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AnnotationCMPEnabled = "akuity.io/enabled"
	AnnotationCMPImage   = "akuity.io/image"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ConfigManagementPlugin is the Schema for the ConfigManagementPlugin API
type ConfigManagementPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PluginSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ConfigManagementPluginList contains a list of ConfigManagementPlugin
type ConfigManagementPluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigManagementPlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigManagementPlugin{}, &ConfigManagementPluginList{})
}

type PluginSpec struct {
	Version          string      `json:"version,omitempty"`
	Init             *Command    `json:"init,omitempty"`
	Generate         *Command    `json:"generate,omitempty"`
	Discover         *Discover   `json:"discover,omitempty"`
	Parameters       *Parameters `json:"parameters,omitempty"`
	PreserveFileMode bool        `json:"preserveFileMode,omitempty"`
}

type Command struct {
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

type Discover struct {
	Find     *Find  `json:"find,omitempty"`
	FileName string `json:"fileName,omitempty"`
}

type Find struct {
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Glob    string   `json:"glob,omitempty"`
}

type Parameters struct {
	Static  []*ParameterAnnouncement `json:"static,omitempty"`
	Dynamic *Dynamic                 `json:"dynamic,omitempty"`
}

type Dynamic struct {
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

type ParameterAnnouncement struct {
	Name           string            `json:"name,omitempty"`
	Title          string            `json:"title,omitempty"`
	Tooltip        string            `json:"tooltip,omitempty"`
	Required       bool              `json:"required,omitempty"`
	ItemType       string            `json:"itemType,omitempty"`
	CollectionType string            `json:"collectionType,omitempty"`
	String_        string            `json:"string,omitempty"`
	Array          []string          `json:"array,omitempty"`
	Map            map[string]string `json:"map,omitempty"`
}
