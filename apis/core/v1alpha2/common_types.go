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
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// LocalReference is a same-namespace reference by name.
//
// Cross-namespace references are not supported in v1alpha2; the
// namespace of the target is always the namespace of the managed
// resource holding the reference.
type LocalReference struct {
	// Name of the referenced object. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// LocalSecretKeySelector selects a specific key from a Secret in the
// same namespace as the referring managed resource.
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

// ResourceStatusCode captures the Akuity API status code + message pair
// exposed on most observable resources.
type ResourceStatusCode struct {
	// Code reported by the Akuity API.
	Code int32 `json:"code,omitempty"`
	// Message reported by the Akuity API.
	Message string `json:"message,omitempty"`
}

// NamedLocalSecretReference binds a gateway-facing credential name to a
// kube Secret in the same namespace as the referring managed resource.
// The Secret's data keys are forwarded to the Akuity gateway verbatim
// under the Name slot (e.g. repoCredentialSecrets["repo-github"] =
// {"url": "...", "password": "..."}).
type NamedLocalSecretReference struct {
	// Name is the identifier under which this Secret's data is sent
	// to the Akuity gateway. For ArgoCD repo credentials this must
	// start with "repo-"; per-field CEL rules enforce a stricter
	// pattern. The MaxLength matches Kubernetes object-name bounds
	// and keeps CEL cost estimates within the apiserver's budget.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// SecretRef points at a Secret in the same namespace as the
	// referring managed resource. All keys of that Secret are
	// forwarded to the gateway verbatim.
	// +kubebuilder:validation:Required
	SecretRef xpv1.LocalSecretReference `json:"secretRef"`
}

// KargoRepoCredentialSecretRef binds a Kargo repository-credential slot
// to a kube Secret in the same namespace as the referring managed
// resource. Mirrors the ArgoCD NamedLocalSecretReference pattern — no
// plaintext on the MR spec — but carries the extra Kargo-specific
// identity bits (project namespace + credential type) needed to route
// the resulting Secret into the correct Kargo project on the gateway.
type KargoRepoCredentialSecretRef struct {
	// Name is the Secret name written into the Kargo project.
	// Must be DNS-1123-compatible. The slot uniquely identifies the
	// credential within its ProjectNamespace.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9][a-z0-9-]*$`
	Name string `json:"name"`

	// ProjectNamespace is the Kargo project namespace the credential
	// belongs to. Kargo enforces DNS-1123 naming on project
	// namespaces; the Pattern here mirrors that.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9][a-z0-9-]*$`
	ProjectNamespace string `json:"projectNamespace"`

	// CredType selects the Kargo credential family. Stamped onto the
	// Secret as the kargo.akuity.io/cred-type label before submission.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=git;helm;generic;image
	CredType string `json:"credType"`

	// SecretRef points at a Secret in the same namespace as the
	// referring managed resource. All keys of that Secret are
	// forwarded to the gateway verbatim under the slot Name.
	// +kubebuilder:validation:Required
	SecretRef xpv1.LocalSecretReference `json:"secretRef"`
}
