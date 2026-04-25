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

// KargoInstanceParameters are the configurable fields of a Kargo
// instance.
//
// CEL rules:
//   - dexConfigSecretRef and dexConfigSecret (both on
//     kargo.oidcConfig) are mutually exclusive. Callers that inlined
//     the secret map can keep doing so; new deployments should use
//     dexConfigSecretRef so plaintext never lives on the managed
//     resource spec.
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
type KargoInstanceParameters struct {
	// Name of the Kargo instance in the Akuity Platform. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Workspace is the ID of the Akuity workspace this Kargo instance
	// belongs to. The Akuity API's ApplyKargoInstance / PatchKargoInstance
	// / DeleteInstance HTTP routes are all scoped under a workspace; the
	// workspace-less compat path is GET-only. Required when the target
	// organization has more than one workspace; for single-workspace
	// organizations the portal's default workspace is used when left
	// unset (controller discovers it on first Observe and records the
	// resolved ID on status.atProvider.workspaceId).
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
	// resources below.
	// The uniqueness rule is written with exists_one rather than
	// size(self.map(...)) == size(self): CEL's .map() returns a list
	// (not a set) so the size comparison never catches duplicates.
	// exists_one is O(n²), kept in budget by the MaxItems=128 cap.
	// +optional
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(i, self.exists_one(j, j.projectNamespace == i.projectNamespace && j.name == i.name))",message="kargoRepoCredentialSecretRefs entries must have unique (projectNamespace, name) pairs"
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

	// ConfigMapHash is the SHA256 of the last-applied
	// spec.forProvider.kargoConfigMap key/value set. Observe compares
	// the current desired digest against this to detect key removals
	// — a case the export-based subset check misses. An empty value
	// means either the field has never been applied or the last apply
	// sent an explicit empty payload (tombstone).
	ConfigMapHash string `json:"configMapHash,omitempty"`

	// RepoCredsAppliedAt records the wall-clock time of the most
	// recent Apply that included spec.forProvider.kargoRepoCredentialSecretRefs.
	// The Kargo Export response has no repo_credentials field, so
	// the controller periodically forces a re-Apply past a TTL to
	// self-heal out-of-band deletions that neither the SecretHash
	// nor the Export-based drift check can see.
	RepoCredsAppliedAt *metav1.Time `json:"repoCredsAppliedAt,omitempty"`

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
