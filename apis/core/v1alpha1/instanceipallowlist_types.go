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

package v1alpha1

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// InstanceIpAllowListParameters manage the ipAllowList field of an
// ArgoCD instance. The underlying PatchInstance endpoint keys by ID;
// callers can supply the ID directly on InstanceID or point at an
// Instance managed resource in the same namespace via InstanceRef, in
// which case the controller resolves the ID from the Instance's
// Status.AtProvider.ID field.
//
// +kubebuilder:validation:XValidation:rule="has(self.instanceId) != has(self.instanceRef)",message="exactly one of instanceId or instanceRef must be set"
// +kubebuilder:validation:XValidation:rule="self.instanceId == oldSelf.instanceId && has(self.instanceRef) == has(oldSelf.instanceRef) && (!has(self.instanceRef) || self.instanceRef.name == oldSelf.instanceRef.name)",message="instanceId/instanceRef are immutable"
type InstanceIpAllowListParameters struct {
	// InstanceID references the target ArgoCD Instance by its opaque
	// Akuity ID. Mutually exclusive with InstanceRef.
	// +optional
	InstanceID string `json:"instanceId,omitempty"`

	// InstanceRef references the target ArgoCD Instance by name in the
	// same namespace as this InstanceIpAllowList. The controller reads
	// the referenced Instance's Status.AtProvider.ID to resolve the
	// underlying Akuity ID. Mutually exclusive with InstanceID.
	// +optional
	InstanceRef *LocalReference `json:"instanceRef,omitempty"`

	// AllowList is the set of IP/CIDR entries to enforce on the
	// instance.
	// +optional
	AllowList []*crossplanetypes.IPAllowListEntry `json:"allowList,omitempty"`
}

// InstanceIpAllowListObservation reflects the observed IP allow list
// on the referenced ArgoCD Instance.
type InstanceIpAllowListObservation struct {
	// AllowList is the set of IP/CIDR entries currently enforced on
	// the instance.
	AllowList []*crossplanetypes.IPAllowListEntry `json:"allowList,omitempty"`

	// InstanceID is the resolved opaque Akuity ID of the target
	// Instance, cached on first successful Observe so Delete can clear
	// the remote allow list even if the referenced Instance MR has
	// already been removed.
	InstanceID string `json:"instanceId,omitempty"`
}

// An InstanceIpAllowListSpec defines the desired state of an
// InstanceIpAllowList.
type InstanceIpAllowListSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       InstanceIpAllowListParameters `json:"forProvider"`
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
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akuity},shortName=ipallow
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
