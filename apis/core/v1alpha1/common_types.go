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

import xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

// LocalReference is a cluster-wide reference to another managed
// resource by name. Cluster-scoped MRs in v1alpha1 do not live in
// a namespace, so the referent is looked up by global name across
// the cluster.
type LocalReference struct {
	// Name of the referenced object. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// ResourceStatusCode captures the Akuity API status code + message pair
// exposed on most observable resources.
type ResourceStatusCode struct {
	// Code reported by the Akuity API.
	Code int32 `json:"code,omitempty"`
	// Message reported by the Akuity API.
	Message string `json:"message,omitempty"`
}

// NamedSecretReference binds a gateway-facing credential name to a
// namespaced kube Secret. If Name is omitted, SecretRef.Name is used as
// the gateway credential name. Resource-specific controllers validate
// the effective name against the destination's rules.
type NamedSecretReference struct {
	// Name is the optional identifier under which this Secret's data is
	// sent to the Akuity gateway. When omitted, SecretRef.Name is used.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name,omitempty"`

	// SecretRef points at a namespaced Secret. All keys of that Secret
	// are forwarded to the gateway verbatim.
	// OpenAPI required only enforces field presence, so the CEL size checks
	// reject empty name/namespace strings explicitly.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="has(self.name) && size(self.name) > 0 && has(self.namespace) && size(self.namespace) > 0",message="secretRef.name and secretRef.namespace are required"
	SecretRef xpv1.SecretReference `json:"secretRef"`
}

func (r NamedSecretReference) CredentialName() string {
	if r.Name != "" {
		return r.Name
	}
	return r.SecretRef.Name
}

// KargoRepoCredentialSecretRef binds a Kargo repository-credential slot
// to a kube Secret. Mirrors the ArgoCD NamedSecretReference
// pattern — no plaintext on the MR spec — but carries the extra
// Kargo-specific identity bits (project namespace + credential type)
// needed to route the resulting Secret into the correct Kargo project
// on the gateway.
type KargoRepoCredentialSecretRef struct {
	NamedSecretReference `json:",inline"`

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
}
