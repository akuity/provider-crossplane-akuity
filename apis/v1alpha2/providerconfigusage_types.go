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

	xpcommon "github.com/crossplane/crossplane-runtime/v2/apis/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +kubebuilder:object:root=true

// A ProviderConfigUsage records that a managed resource uses a
// ProviderConfig or ClusterProviderConfig. The embedded
// TypedProviderConfigUsage payload tracks both the provider-config
// reference (including Kind, which distinguishes namespaced from
// cluster-scoped) and the managed-resource reference.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="CONFIG-KIND",type="string",JSONPath=".providerConfigRef.kind"
// +kubebuilder:printcolumn:name="CONFIG-NAME",type="string",JSONPath=".providerConfigRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".resourceRef.name"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,provider,akuity}
type ProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpcommon.TypedProviderConfigUsage `json:",inline"`
}

// +kubebuilder:object:root=true

// ProviderConfigUsageList contains a list of ProviderConfigUsage.
type ProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfigUsage `json:"items"`
}

// ProviderConfigUsage type metadata.
var (
	ProviderConfigUsageKind             = reflect.TypeOf(ProviderConfigUsage{}).Name()
	ProviderConfigUsageGroupKind        = schema.GroupKind{Group: Group, Kind: ProviderConfigUsageKind}.String()
	ProviderConfigUsageKindAPIVersion   = ProviderConfigUsageKind + "." + SchemeGroupVersion.String()
	ProviderConfigUsageGroupVersionKind = SchemeGroupVersion.WithKind(ProviderConfigUsageKind)

	ProviderConfigUsageListKind             = reflect.TypeOf(ProviderConfigUsageList{}).Name()
	ProviderConfigUsageListGroupKind        = schema.GroupKind{Group: Group, Kind: ProviderConfigUsageListKind}.String()
	ProviderConfigUsageListKindAPIVersion   = ProviderConfigUsageListKind + "." + SchemeGroupVersion.String()
	ProviderConfigUsageListGroupVersionKind = SchemeGroupVersion.WithKind(ProviderConfigUsageListKind)
)

func init() {
	SchemeBuilder.Register(&ProviderConfigUsage{}, &ProviderConfigUsageList{})
}
