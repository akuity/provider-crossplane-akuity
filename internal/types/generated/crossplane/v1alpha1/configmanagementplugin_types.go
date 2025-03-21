// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2023 Akuity, Inc.
*/

package v1alpha1

// +kubebuilder:object:generate=true
type ConfigManagementPlugin struct {
	Enabled bool       `json:"enabled"`
	Image   string     `json:"image"`
	Spec    PluginSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:generate=true
type ConfigManagementPluginList struct {
	Items []ConfigManagementPlugin `json:"items"`
}

// +kubebuilder:object:generate=true
type PluginSpec struct {
	Version          string      `json:"version,omitempty"`
	Init             *Command    `json:"init,omitempty"`
	Generate         *Command    `json:"generate,omitempty"`
	Discover         *Discover   `json:"discover,omitempty"`
	Parameters       *Parameters `json:"parameters,omitempty"`
	PreserveFileMode *bool       `json:"preserveFileMode,omitempty"`
}

// +kubebuilder:object:generate=true
type Command struct {
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// +kubebuilder:object:generate=true
type Discover struct {
	Find     *Find  `json:"find,omitempty"`
	FileName string `json:"fileName,omitempty"`
}

// +kubebuilder:object:generate=true
type Find struct {
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Glob    string   `json:"glob,omitempty"`
}

// +kubebuilder:object:generate=true
type Parameters struct {
	Static  []*ParameterAnnouncement `json:"static,omitempty"`
	Dynamic *Dynamic                 `json:"dynamic,omitempty"`
}

// +kubebuilder:object:generate=true
type Dynamic struct {
	Command []string `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// +kubebuilder:object:generate=true
type ParameterAnnouncement struct {
	Name           string            `json:"name,omitempty"`
	Title          string            `json:"title,omitempty"`
	Tooltip        string            `json:"tooltip,omitempty"`
	Required       *bool             `json:"required,omitempty"`
	ItemType       string            `json:"itemType,omitempty"`
	CollectionType string            `json:"collectionType,omitempty"`
	String_        string            `json:"string,omitempty"`
	Array          []string          `json:"array,omitempty"`
	Map            map[string]string `json:"map,omitempty"`
}
