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

// A ClusterProviderConfigSpec defines the desired state of a
// cluster-scoped ClusterProviderConfig.
type ClusterProviderConfigSpec struct {
	// CredentialsSecretRef references a Secret, anywhere in the
	// cluster, that holds the Akuity credentials. Required.
	// +kubebuilder:validation:Required
	CredentialsSecretRef xpv1.SecretKeySelector `json:"credentialsSecretRef"`

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

// A ClusterProviderConfigStatus reflects the observed state of a
// ClusterProviderConfig.
type ClusterProviderConfigStatus struct {
	xpv1.ProviderConfigStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A ClusterProviderConfig is a cluster-scoped configuration for the
// Akuity provider. Namespaced managed resources may reference either
// a ProviderConfig (in their own namespace) or a ClusterProviderConfig.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentialsSecretRef.name",priority=1
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,akuity}
type ClusterProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterProviderConfigSpec   `json:"spec"`
	Status ClusterProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterProviderConfigList contains a list of ClusterProviderConfig.
type ClusterProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterProviderConfig `json:"items"`
}

// ClusterProviderConfig type metadata.
var (
	ClusterProviderConfigKind             = reflect.TypeOf(ClusterProviderConfig{}).Name()
	ClusterProviderConfigGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterProviderConfigKind}.String()
	ClusterProviderConfigKindAPIVersion   = ClusterProviderConfigKind + "." + SchemeGroupVersion.String()
	ClusterProviderConfigGroupVersionKind = SchemeGroupVersion.WithKind(ClusterProviderConfigKind)
)

func init() {
	SchemeBuilder.Register(&ClusterProviderConfig{}, &ClusterProviderConfigList{})
}
