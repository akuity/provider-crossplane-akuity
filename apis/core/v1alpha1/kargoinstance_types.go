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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// KargoConfigMapAllowedKeys mirrors the JSON/proto field names on the
// platform's KargoApiCM proto (akuity-platform/pkg/api/gen/kargo/v1/kargo.pb.go,
// search for `type KargoApiCM struct`). The lowerCamel spellings are
// canonical in platform exports and docs; snake_case proto names are
// accepted for compatibility with protojson. Any key outside this set
// causes ApplyKargoInstance on the platform to fail strict protojson
// unmarshal because the receiver decodes the patched instance into
// KargoApiCM with no DiscardUnknown. The CRD CEL rule on
// KargoInstanceParameters enforces the same set at admission so users
// get an immediate error instead of a cryptic Apply retry storm.
//
// The Go list and CEL strings are hand-typed in three places: this
// slice, the allowlist CEL, and the alias-conflict CEL. Updating one
// requires updating all three, both keyed off the same upstream proto.
var KargoConfigMapAllowedKeys = []string{
	"adminAccountEnabled",
	"adminAccountTokenTtl",
	"admin_account_enabled",
	"admin_account_token_ttl",
}

// KargoInstanceParameters are the configurable fields of a Kargo
// instance.
//
// CEL rules:
//   - dexConfigSecretRef and dexConfigSecret (both on
//     kargo.oidcConfig) are mutually exclusive. Callers that inlined
//     the secret map can keep doing so; new deployments should use
//     dexConfigSecretRef so plaintext never lives on the managed
//     resource spec.
//   - kargoConfigMap keys are constrained to the platform's KargoApiCM
//     proto field set (see kargoConfigMapAllowedKeys above). Any other
//     key would crash ApplyKargoInstance's strict protojson unmarshal
//     server-side and hot-loop the reconciler on retries.
//
// v1/Secret manifests are forbidden inside resources, but the check
// lives in the controller (splitKargoResources) rather than CEL: the
// field is schemaless/preserve-unknown-fields, which hides its element
// shape from the apiserver's CEL compiler and would fail CRD install
// with "undefined field 'resources'". Repository credentials must
// flow through kargoRepoCredentialSecretRefs (typed refs); inline
// v1/Secret entries in resources are rejected at reconcile time.
//
// +kubebuilder:validation:XValidation:rule="!has(self.kargo.oidcConfig) || !has(self.kargo.oidcConfig.dexConfigSecretRef) || !has(self.kargo.oidcConfig.dexConfigSecret) || size(self.kargo.oidcConfig.dexConfigSecret) == 0",message="set either kargo.oidcConfig.dexConfigSecretRef or kargo.oidcConfig.dexConfigSecret, not both"
// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="name is immutable"
// +kubebuilder:validation:XValidation:rule="!has(self.kargoConfigMap) || self.kargoConfigMap.all(k, k in ['adminAccountEnabled', 'adminAccountTokenTtl', 'admin_account_enabled', 'admin_account_token_ttl'])",message="kargoConfigMap accepts only these keys: adminAccountEnabled, adminAccountTokenTtl, admin_account_enabled, admin_account_token_ttl. Other keys are rejected before Apply."
// +kubebuilder:validation:XValidation:rule="!has(self.kargoConfigMap) || !('adminAccountEnabled' in self.kargoConfigMap && 'admin_account_enabled' in self.kargoConfigMap) && !('adminAccountTokenTtl' in self.kargoConfigMap && 'admin_account_token_ttl' in self.kargoConfigMap)",message="kargoConfigMap must not set both lowerCamel and snake_case aliases for the same key"
type KargoInstanceParameters struct {
	// Name of the Kargo instance in the Akuity Platform. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Workspace is the Akuity workspace this Kargo instance belongs to. The
	// preferred value is the workspace ID because workspace-scoped gateway calls
	// are routed by ID. A workspace name is also accepted and resolved before
	// gateway calls. When omitted, the organization default workspace is used on
	// create. The canonical workspace ID is reported in
	// status.atProvider.workspace.
	//
	// +optional
	Workspace string `json:"workspace,omitempty"`

	// Kargo describes the Kargo configuration. Required.
	// +kubebuilder:validation:Required
	Kargo crossplanetypes.KargoSpec `json:"kargo"`

	// KargoConfigMap sets keys in the kargo-cm ConfigMap shipped to
	// the Kargo control plane.
	// +optional
	KargoConfigMap map[string]string `json:"kargoConfigMap,omitempty"`

	// KargoSecretRef references a namespaced Secret whose data is sent
	// verbatim as the kargo-secret payload (OIDC client secrets,
	// webhook tokens, admin bootstrap). Removing this ref stops
	// applying the platform-side Secret, but does not delete it from
	// the Akuity platform.
	// +optional
	// +kubebuilder:validation:XValidation:rule="has(self.name) && size(self.name) > 0 && has(self.__namespace__) && size(self.__namespace__) > 0",message="kargoSecretRef.name and kargoSecretRef.namespace are required"
	KargoSecretRef *xpv1.SecretReference `json:"kargoSecretRef,omitempty"`

	// KargoRepoCredentialSecretRefs registers repository credentials
	// with the Kargo gateway. Each entry's SecretRef points at a
	// namespaced Secret; the controller synthesises a
	// labelled Kargo-shaped Secret (kargo.akuity.io/cred-type=<type>)
	// named after the effective slot, writes it into the effective
	// Kargo project namespace, and forwards it to ApplyKargoInstance.
	// If an entry omits Name, the controller uses SecretRef.Name as
	// the slot. If ProjectNamespace is omitted, the controller uses
	// SecretRef.Namespace. If CredType is omitted, the controller uses
	// the referenced Secret's kargo.akuity.io/cred-type label.
	// Plaintext never lives on the managed resource spec.
	//
	// Drift: this field is write-only on the Kargo gateway — the
	// Export response does not return repo_credentials — so rotation
	// participates in drift via the SecretHash on
	// status.atProvider. Removal from spec does NOT delete the
	// Secret on the gateway; delete it through the Akuity platform
	// UI or API if it should be removed.
	// Explicit Name values are validated at admission. When Name is
	// omitted, the controller validates the effective SecretRef.Name
	// and duplicate (effective project namespace, effective name) pairs at
	// reconcile time; the upstream Crossplane SecretReference schema
	// intentionally does not bound name length, so equivalent CEL rules
	// would exceed the apiserver's CRD cost budget.
	// +optional
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(r, !has(r.name) || r.name.matches('^[a-z0-9][a-z0-9-]*$'))",message="kargoRepoCredentialSecretRefs names must match ^[a-z0-9][a-z0-9-]*$ when set"
	KargoRepoCredentialSecretRefs []KargoRepoCredentialSecretRef `json:"kargoRepoCredentialSecretRefs,omitempty"`

	// Resources carries raw YAML manifests for declarative Kargo
	// child resources (Projects, Warehouses, Stages,
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
	// Additive semantics: removing an entry from this list does NOT
	// delete the corresponding resource on the Akuity platform. The
	// controller does not enable ApplyKargoInstance's
	// PruneResourceTypes because out-of-band resources managed via
	// the Akuity UI or other tooling would be deleted as collateral
	// damage. To remove a resource, delete it via the Akuity
	// platform UI or API. See PARITY_PLAN.md for the rationale.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Resources []runtime.RawExtension `json:"resources,omitempty"`
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

	// Kargo is the observed Kargo configuration, mirroring
	// spec.forProvider.kargo on the most recent reconcile. Included
	// so atProvider is a faithful reflection of forProvider for
	// compositions and dashboards (parity with InstanceObservation).
	Kargo crossplanetypes.KargoSpec `json:"kargo,omitempty"`

	// SecretHash is the SHA256 of the concatenation of every resolved
	// Secret referenced by spec.forProvider on the most recent Apply.
	// The Kargo gateway masks secret contents on Get, so the
	// controller uses this stored digest as the drift signal for
	// Secret rotation. Stored in status rather than an annotation
	// because managed.Reconciler persists status after Create/Update
	// but does not persist arbitrary metadata.
	SecretHash string `json:"secretHash,omitempty"`

	// Workspace is the canonical Akuity workspace ID this Kargo
	// instance belongs to. The controller stamps it on the first
	// reconcile after resolving the organisation's default workspace
	// (when spec.forProvider.workspace is empty) so subsequent polls
	// route ApplyKargoInstance / ExportKargoInstance / DeleteKargoInstance
	// to the correct workspace-scoped HTTP path without a fresh
	// ListWorkspaces round-trip. The HTTP routes 404 when the
	// workspace_id template segment is empty, which previously hot-looped
	// portal-server (~350 wasted writes / 12 minutes) on first-create
	// for any KargoInstance that omitted spec.workspace.
	Workspace string `json:"workspace,omitempty"`
}

// A KargoInstanceSpec defines the desired state of a Kargo instance.
type KargoInstanceSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       KargoInstanceParameters `json:"forProvider"`
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
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akuity},shortName=kinst
type KargoInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KargoInstanceSpec   `json:"spec"`
	Status KargoInstanceStatus `json:"status,omitempty"`
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
