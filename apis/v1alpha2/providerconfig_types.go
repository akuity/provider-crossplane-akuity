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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// LocalSecretKeySelector selects a specific key from a Secret living in
// the same namespace as the ProviderConfig that references it. Used by
// the namespaced ProviderConfig; the cluster-scoped
// ClusterProviderConfig uses xpv1.SecretKeySelector which carries an
// explicit namespace.
type LocalSecretKeySelector struct {
	// Name of the Secret. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key within the Secret. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// A ProviderConfigSpec defines the desired state of a (namespaced)
// ProviderConfig.
type ProviderConfigSpec struct {
	// CredentialsSecretRef references a Secret in the same namespace as
	// this ProviderConfig that holds the Akuity credentials. Required.
	// +kubebuilder:validation:Required
	CredentialsSecretRef LocalSecretKeySelector `json:"credentialsSecretRef"`

	// OrganizationID of the Akuity organization these credentials
	// belong to. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	OrganizationID string `json:"organizationId"`

	// ServerURL is the Akuity Platform API endpoint. Defaults to
	// https://akuity.cloud when unset.
	// +optional
	ServerURL string `json:"serverUrl,omitempty"`

	// SkipTLSVerify disables TLS verification when talking to the
	// Akuity Platform API. Defaults to false.
	// +optional
	SkipTLSVerify bool `json:"skipTlsVerify,omitempty"`
}

// A ProviderConfigStatus reflects the observed state of a
// ProviderConfig.
type ProviderConfigStatus struct {
	xpv1.ProviderConfigStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A ProviderConfig configures the Akuity provider.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentialsSecretRef.name",priority=1
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,provider,akuity}
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// ProviderConfig type metadata.
var (
	ProviderConfigKind             = reflect.TypeOf(ProviderConfig{}).Name()
	ProviderConfigGroupKind        = schema.GroupKind{Group: Group, Kind: ProviderConfigKind}.String()
	ProviderConfigKindAPIVersion   = ProviderConfigKind + "." + SchemeGroupVersion.String()
	ProviderConfigGroupVersionKind = SchemeGroupVersion.WithKind(ProviderConfigKind)
)

func init() {
	SchemeBuilder.Register(&ProviderConfig{}, &ProviderConfigList{})
}
