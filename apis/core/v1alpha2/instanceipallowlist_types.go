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

package v1alpha2

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// InstanceIpAllowListParameters manage the ipAllowList field of an
// ArgoCD instance. This resource is a convenience wrapper that lets a
// separate controller own the allow list independently from the owning
// Instance MR (supporting ownership-by-reference patterns).
type InstanceIpAllowListParameters struct {
	// InstanceRef references the target ArgoCD Instance by name in the
	// same namespace as this InstanceIpAllowList. The Akuity apply path
	// keys by instance name, so referencing by name is the only
	// supported resolution mode.
	// +kubebuilder:validation:Required
	InstanceRef *LocalReference `json:"instanceRef"`

	// AllowList is the set of IP/CIDR entries to enforce on the
	// instance.
	// +optional
	AllowList []*IPAllowListEntry `json:"allowList,omitempty"`
}

// InstanceIpAllowListObservation reflects the observed IP allow list
// on the referenced ArgoCD Instance.
type InstanceIpAllowListObservation struct {
	// AllowList is the set of IP/CIDR entries currently enforced on
	// the instance.
	AllowList []*IPAllowListEntry `json:"allowList,omitempty"`
}

// An InstanceIpAllowListSpec defines the desired state of an
// InstanceIpAllowList.
type InstanceIpAllowListSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              InstanceIpAllowListParameters `json:"forProvider"`
}

// An InstanceIpAllowListStatus represents the observed state of an
// InstanceIpAllowList.
type InstanceIpAllowListStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          InstanceIpAllowListObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// An InstanceIpAllowList manages the IP allow list of an ArgoCD
// Instance.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,akuity}
type InstanceIpAllowList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceIpAllowListSpec   `json:"spec"`
	Status InstanceIpAllowListStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceIpAllowListList contains a list of InstanceIpAllowList.
type InstanceIpAllowListList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstanceIpAllowList `json:"items"`
}

// InstanceIpAllowList type metadata.
var (
	InstanceIpAllowListKind             = reflect.TypeOf(InstanceIpAllowList{}).Name()
	InstanceIpAllowListGroupKind        = schema.GroupKind{Group: Group, Kind: InstanceIpAllowListKind}.String()
	InstanceIpAllowListKindAPIVersion   = InstanceIpAllowListKind + "." + SchemeGroupVersion.String()
	InstanceIpAllowListGroupVersionKind = SchemeGroupVersion.WithKind(InstanceIpAllowListKind)
)

func init() {
	SchemeBuilder.Register(&InstanceIpAllowList{}, &InstanceIpAllowListList{})
}
