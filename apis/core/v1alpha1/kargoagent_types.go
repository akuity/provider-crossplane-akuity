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

// KargoAgentParameters are the configurable fields of a KargoAgent.
//
// +kubebuilder:validation:XValidation:rule="has(self.kargoInstanceId) || has(self.kargoInstanceRef)",message="kargoInstanceId or kargoInstanceRef must be set"
// +kubebuilder:validation:XValidation:rule="(!has(oldSelf.kargoInstanceId) || (has(self.kargoInstanceId) && self.kargoInstanceId == oldSelf.kargoInstanceId)) && (!has(oldSelf.kargoInstanceRef) || (has(self.kargoInstanceRef) && self.kargoInstanceRef.name == oldSelf.kargoInstanceRef.name))",message="kargoInstanceId/kargoInstanceRef are immutable"
// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="name is immutable"
type KargoAgentParameters struct {
	// KargoInstanceID references the owning Kargo instance by ID.
	// Either KargoInstanceID or KargoInstanceRef must be set.
	// +optional
	KargoInstanceID string `json:"kargoInstanceId,omitempty"`

	// KargoInstanceRef references the owning Kargo instance by name in
	// the same namespace as this KargoAgent.
	// +optional
	KargoInstanceRef *LocalReference `json:"kargoInstanceRef,omitempty"`

	// Workspace is the Kargo project/workspace the agent is bound to.
	// +optional
	Workspace string `json:"workspace,omitempty"`

	// Name of the agent. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace in the managed cluster where the agent is installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Labels applied to the agent.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations applied to the agent.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// KargoAgentSpec carries the agent configuration payload
	// (description + data). The wrapper groups gateway-owned fields
	// so forProvider siblings (name, namespace, refs, labels) stay
	// cleanly separated from what is sent to the Akuity gateway.
	// Mirrors the Cluster pattern where payload lives under
	// clusterSpec alongside hoisted k8s/Crossplane identity fields.
	// +optional
	KargoAgentSpec crossplanetypes.KargoAgentSpec `json:"kargoAgentSpec,omitempty"`
}

// KargoAgentObservation are the observable fields of a KargoAgent.
type KargoAgentObservation struct {
	// ID assigned by the Akuity Platform.
	ID string `json:"id,omitempty"`
	// Name of the agent as reported by the Akuity Platform.
	Name string `json:"name,omitempty"`
	// Workspace the agent is bound to.
	Workspace string `json:"workspace,omitempty"`
	// HealthStatus is the agent health.
	HealthStatus ResourceStatusCode `json:"healthStatus,omitempty"`
	// ReconciliationStatus is the agent reconciliation status.
	ReconciliationStatus ResourceStatusCode `json:"reconciliationStatus,omitempty"`

	// KargoAgentSpec is the observed KargoAgent payload (description +
	// data), mirroring spec.forProvider.kargoAgentSpec on the most
	// recent reconcile.
	KargoAgentSpec crossplanetypes.KargoAgentSpec `json:"kargoAgentSpec,omitempty"`
}

// A KargoAgentSpec defines the desired state of a KargoAgent.
type KargoAgentSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       KargoAgentParameters `json:"forProvider"`
}

// A KargoAgentStatus represents the observed state of a KargoAgent.
type KargoAgentStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          KargoAgentObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A KargoAgent is a Kargo agent installed in a managed Kubernetes
// cluster.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akuity},shortName=kagent
type KargoAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KargoAgentSpec   `json:"spec"`
	Status KargoAgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KargoAgentList contains a list of KargoAgent.
type KargoAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KargoAgent `json:"items"`
}

// KargoAgent type metadata.
var (
	KargoAgentKind             = reflect.TypeOf(KargoAgent{}).Name()
	KargoAgentGroupKind        = schema.GroupKind{Group: Group, Kind: KargoAgentKind}.String()
	KargoAgentKindAPIVersion   = KargoAgentKind + "." + SchemeGroupVersion.String()
	KargoAgentGroupVersionKind = SchemeGroupVersion.WithKind(KargoAgentKind)
)

func init() {
	SchemeBuilder.Register(&KargoAgent{}, &KargoAgentList{})
}
