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
)

// KargoDefaultShardAgentParameters pin the default shard agent of a
// Kargo instance. The underlying PatchKargoInstance endpoint keys by
// ID; callers can supply the ID directly on KargoInstanceID or point
// at a KargoInstance managed resource in the same namespace via
// KargoInstanceRef, in which case the controller resolves the ID from
// the KargoInstance's Status.AtProvider.ID field.
//
// +kubebuilder:validation:XValidation:rule="has(self.kargoInstanceId) || has(self.kargoInstanceRef)",message="kargoInstanceId or kargoInstanceRef must be set"
// +kubebuilder:validation:XValidation:rule="(!has(oldSelf.kargoInstanceId) || (has(self.kargoInstanceId) && self.kargoInstanceId == oldSelf.kargoInstanceId)) && (!has(oldSelf.kargoInstanceRef) || (has(self.kargoInstanceRef) && self.kargoInstanceRef.name == oldSelf.kargoInstanceRef.name))",message="kargoInstanceId/kargoInstanceRef are immutable"
type KargoDefaultShardAgentParameters struct {
	// KargoInstanceID references the owning Kargo instance by its
	// opaque Akuity ID. Mutually exclusive with KargoInstanceRef.
	// +optional
	KargoInstanceID string `json:"kargoInstanceId,omitempty"`

	// KargoInstanceRef references the owning Kargo instance by name
	// in the same namespace as this KargoDefaultShardAgent. The
	// controller reads the referenced KargoInstance's
	// Status.AtProvider.ID to resolve the underlying Akuity ID.
	// Mutually exclusive with KargoInstanceID.
	// +optional
	KargoInstanceRef *LocalReference `json:"kargoInstanceRef,omitempty"`

	// AgentName is the name of the shard agent to promote as default.
	// Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	AgentName string `json:"agentName"`
}

// KargoDefaultShardAgentObservation reflects the observed default
// shard agent for the referenced Kargo instance.
type KargoDefaultShardAgentObservation struct {
	// AgentName is the default shard agent currently set on the Kargo
	// instance.
	AgentName string `json:"agentName,omitempty"`

	// KargoInstanceID is the resolved opaque Akuity ID of the target
	// Kargo instance, cached on first successful Observe so Delete can
	// clear the remote field even if the referenced KargoInstance MR
	// has already been removed.
	KargoInstanceID string `json:"kargoInstanceId,omitempty"`
}

// A KargoDefaultShardAgentSpec defines the desired state of a
// KargoDefaultShardAgent.
type KargoDefaultShardAgentSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       KargoDefaultShardAgentParameters `json:"forProvider"`
}

// A KargoDefaultShardAgentStatus represents the observed state of a
// KargoDefaultShardAgent.
type KargoDefaultShardAgentStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          KargoDefaultShardAgentObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A KargoDefaultShardAgent pins the defaultShardAgent field of a Kargo
// instance.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akuity},shortName=kdsa
type KargoDefaultShardAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KargoDefaultShardAgentSpec   `json:"spec"`
	Status KargoDefaultShardAgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KargoDefaultShardAgentList contains a list of KargoDefaultShardAgent.
type KargoDefaultShardAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KargoDefaultShardAgent `json:"items"`
}

// KargoDefaultShardAgent type metadata.
var (
	KargoDefaultShardAgentKind             = reflect.TypeOf(KargoDefaultShardAgent{}).Name()
	KargoDefaultShardAgentGroupKind        = schema.GroupKind{Group: Group, Kind: KargoDefaultShardAgentKind}.String()
	KargoDefaultShardAgentKindAPIVersion   = KargoDefaultShardAgentKind + "." + SchemeGroupVersion.String()
	KargoDefaultShardAgentGroupVersionKind = SchemeGroupVersion.WithKind(KargoDefaultShardAgentKind)
)

func init() {
	SchemeBuilder.Register(&KargoDefaultShardAgent{}, &KargoDefaultShardAgentList{})
}
