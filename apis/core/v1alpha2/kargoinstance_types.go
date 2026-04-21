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

// KargoInstanceParameters are the configurable fields of a Kargo
// instance.
type KargoInstanceParameters struct {
	// Name of the Kargo instance in the Akuity Platform. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Spec describes the Kargo configuration. Required.
	// +kubebuilder:validation:Required
	Spec KargoSpec `json:"spec"`
}

// KargoInstanceObservation are the observable fields of a Kargo
// instance.
type KargoInstanceObservation struct {
	// ID assigned by the Akuity Platform.
	ID string `json:"id,omitempty"`
	// Name of the instance.
	Name string `json:"name,omitempty"`
	// Hostname is the public hostname.
	Hostname string `json:"hostname,omitempty"`
	// HealthStatus is the instance health.
	HealthStatus ResourceStatusCode `json:"healthStatus,omitempty"`
	// ReconciliationStatus is the instance reconciliation status.
	ReconciliationStatus ResourceStatusCode `json:"reconciliationStatus,omitempty"`
	// OwnerOrganizationName is the Akuity organization owning the
	// instance.
	OwnerOrganizationName string `json:"ownerOrganizationName,omitempty"`
}

// A KargoInstanceResourceSpec defines the desired state of a Kargo
// instance.
type KargoInstanceResourceSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              KargoInstanceParameters `json:"forProvider"`
}

// A KargoInstanceStatus represents the observed state of a Kargo
// instance.
type KargoInstanceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          KargoInstanceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A KargoInstance is an Akuity-managed Kargo control-plane instance.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,akuity}
type KargoInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KargoInstanceResourceSpec `json:"spec"`
	Status KargoInstanceStatus       `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KargoInstanceList contains a list of KargoInstance.
type KargoInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KargoInstance `json:"items"`
}

// KargoInstance type metadata.
var (
	KargoInstanceKind             = reflect.TypeOf(KargoInstance{}).Name()
	KargoInstanceGroupKind        = schema.GroupKind{Group: Group, Kind: KargoInstanceKind}.String()
	KargoInstanceKindAPIVersion   = KargoInstanceKind + "." + SchemeGroupVersion.String()
	KargoInstanceGroupVersionKind = SchemeGroupVersion.WithKind(KargoInstanceKind)
)

func init() {
	SchemeBuilder.Register(&KargoInstance{}, &KargoInstanceList{})
}
