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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// KargoInstanceParameters are the configurable fields of a Kargo
// instance.
//
// CEL rules:
//   - dexConfigSecretRef and dexConfigSecret (both on
//     spec.oidcConfig) are mutually exclusive. Existing v1alpha2
//     adopters that inlined the secret map can keep doing so; new
//     deployments should use dexConfigSecretRef so plaintext never
//     lives on the managed resource spec.
//   - v1/Secret manifests are forbidden inside kargoResources. The
//     field is schemaless/preserve-unknown-fields, which means the
//     apiserver would otherwise persist inline Secret data in etcd
//     before the controller could reject it at reconcile time.
//     Repository credentials must flow through
//     kargoRepoCredentialSecretRefs (typed refs).
//
// +kubebuilder:validation:XValidation:rule="!has(self.spec.oidcConfig) || !has(self.spec.oidcConfig.dexConfigSecretRef) || !has(self.spec.oidcConfig.dexConfigSecret) || size(self.spec.oidcConfig.dexConfigSecret) == 0",message="set either spec.oidcConfig.dexConfigSecretRef or spec.oidcConfig.dexConfigSecret, not both"
// +kubebuilder:validation:XValidation:rule="!has(self.kargoResources) || self.kargoResources.all(r, !(has(r.apiVersion) && has(r.kind) && r.apiVersion == 'v1' && r.kind == 'Secret'))",message="v1/Secret entries are not accepted in spec.forProvider.kargoResources; use spec.forProvider.kargoRepoCredentialSecretRefs instead"
type KargoInstanceParameters struct {
	// Name of the Kargo instance in the Akuity Platform. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Spec describes the Kargo configuration. Required.
	// +kubebuilder:validation:Required
	Spec KargoSpec `json:"spec"`

	// KargoConfigMap sets keys in the kargo-cm ConfigMap shipped to
	// the Kargo control plane.
	// +optional
	KargoConfigMap map[string]string `json:"kargoConfigMap,omitempty"`

	// KargoSecretRef references a Secret whose data is sent verbatim
	// as the kargo-secret payload (OIDC client secrets, webhook
	// tokens, admin bootstrap).
	// +optional
	KargoSecretRef *xpv1.LocalSecretReference `json:"kargoSecretRef,omitempty"`

	// KargoRepoCredentialSecretRefs registers repository credentials
	// with the Kargo gateway. Each entry's SecretRef points at a
	// Secret in the MR's namespace; the controller synthesises a
	// labelled Kargo-shaped Secret (kargo.akuity.io/cred-type=<type>)
	// named after the slot, writes it into the entry's
	// ProjectNamespace, and forwards it to ApplyKargoInstance.
	// Plaintext never lives on the managed resource spec.
	//
	// Drift: this field is write-only on the Kargo gateway — the
	// Export response does not return repo_credentials — so rotation
	// participates in drift via the SecretHash on
	// status.atProvider. Removal from spec does NOT delete the
	// Secret on the gateway; see the additive-semantics note on
	// KargoResources below.
	//
	// MaxItems caps the CEL cost estimate so kube-apiserver accepts
	// the uniqueness rule; 128 slots is far above realistic usage
	// and can be raised with a CRD bump if a need emerges.
	// +optional
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="size(self.map(e, e.projectNamespace + '/' + e.name)) == size(self)",message="kargoRepoCredentialSecretRefs entries must have unique (projectNamespace, name) pairs"
	KargoRepoCredentialSecretRefs []KargoRepoCredentialSecretRef `json:"kargoRepoCredentialSecretRefs,omitempty"`

	// KargoResources carries raw YAML manifests for declarative
	// Kargo child resources (Projects, Warehouses, Stages,
	// AnalysisTemplates, PromotionTasks, ClusterPromotionTasks).
	// Repository-credential Secrets are NOT accepted here — the
	// parent-level CEL rule rejects v1/Secret entries at admission
	// time so inline credential data cannot land in etcd. Use
	// KargoRepoCredentialSecretRefs (typed refs) so plaintext stays
	// out of the MR spec and rotation flows through SecretHash drift.
	//
	// The controller validates each entry's apiVersion/kind, groups
	// them by kind, and sends them alongside the instance spec on
	// every ApplyKargoInstance call. Write the YAML as an object,
	// not a string; the CRD is preserve-unknown-fields so the
	// payload is opaque to kube-apiserver structural validation
	// (CEL on specific kinds still runs). The Akuity gateway
	// rejects malformed payloads server-side.
	//
	// MaxItems caps the CEL-reachable list length so the cost
	// estimator stays within kube-apiserver's budget; 256 is well
	// above any realistic declarative child count.
	//
	// Additive semantics: removing an entry from this list does NOT
	// delete the corresponding resource on the Akuity platform. The
	// controller does not enable ApplyKargoInstance's
	// PruneResourceTypes because out-of-band resources managed via
	// the Akuity UI or other tooling would be deleted as collateral
	// damage. To remove a resource, delete it via the Akuity
	// platform UI or API. See PARITY_PLAN.md for the rationale.
	// +optional
	// +kubebuilder:validation:MaxItems=256
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	KargoResources []runtime.RawExtension `json:"kargoResources,omitempty"`
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

	// SecretHash is the SHA256 of the concatenation of every resolved
	// Secret referenced by spec.forProvider on the most recent Apply.
	// The Kargo gateway masks secret contents on Get, so the
	// controller uses this stored digest as the drift signal for
	// Secret rotation. Stored in status rather than an annotation
	// because managed.Reconciler persists status after Create/Update
	// but does not persist arbitrary metadata.
	SecretHash string `json:"secretHash,omitempty"`

	// KargoConfigMapHash is the SHA256 of the last-applied
	// spec.forProvider.kargoConfigMap key/value set. Observe compares
	// the current desired digest against this to detect key removals
	// — a case the export-based subset check misses. An empty value
	// means either the field has never been applied or the last apply
	// sent an explicit empty payload (tombstone).
	KargoConfigMapHash string `json:"kargoConfigMapHash,omitempty"`

	// RepoCredsAppliedAt records the wall-clock time of the most
	// recent Apply that included spec.forProvider.kargoRepoCredentialSecretRefs.
	// The Kargo Export response has no repo_credentials field, so
	// the controller periodically forces a re-Apply past a TTL to
	// self-heal out-of-band deletions that neither the SecretHash
	// nor the Export-based drift check can see.
	RepoCredsAppliedAt *metav1.Time `json:"repoCredsAppliedAt,omitempty"`
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
