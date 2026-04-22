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

// InstanceParameters are the configurable fields of an ArgoCD Instance.
//
// CEL rules:
//   - v1/Secret manifests are forbidden inside resources. The field is
//     schemaless/preserve-unknown-fields, so without this rule the
//     apiserver would persist inline Secret data in etcd before the
//     controller could reject it at reconcile time. Secret payloads
//     must flow through the typed *SecretRef fields (argocdSecretRef,
//     repoCredentialSecretRefs, ...).
//
// +kubebuilder:validation:XValidation:rule="!has(self.resources) || self.resources.all(r, !(has(r.apiVersion) && has(r.kind) && r.apiVersion == 'v1' && r.kind == 'Secret'))",message="v1/Secret entries are not accepted in spec.forProvider.resources; use the typed *SecretRef fields instead"
type InstanceParameters struct {
	// Name of the instance in the Akuity Platform. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ArgoCD contains the ArgoCD configuration. Required.
	//
	// Points at the upstream ArgoCDSpec wire type directly. The prior
	// shell-level ArgoCD{Spec ArgoCDSpec} wrapper was pure indirection
	// and has been removed to flatten the user-facing YAML by one
	// level: .spec.forProvider.argocd.* now reaches the wire fields
	// (description, version, shard, instanceSpec) that used to live
	// at .spec.forProvider.argocd.spec.* .
	// +kubebuilder:validation:Required
	ArgoCD *ArgoCDSpec `json:"argocd"`

	// ArgoCDConfigMap sets keys in the argocd-cm ConfigMap.
	// +optional
	ArgoCDConfigMap map[string]string `json:"argocdConfigMap,omitempty"`

	// ArgoCDImageUpdaterConfigMap sets keys in the
	// argocd-image-updater-config ConfigMap.
	// +optional
	ArgoCDImageUpdaterConfigMap map[string]string `json:"argocdImageUpdaterConfigMap,omitempty"`

	// ArgoCDImageUpdaterSSHConfigMap sets keys in the
	// argocd-image-updater-ssh-config ConfigMap.
	// +optional
	ArgoCDImageUpdaterSSHConfigMap map[string]string `json:"argocdImageUpdaterSshConfigMap,omitempty"`

	// ArgoCDNotificationsConfigMap sets keys in the
	// argocd-notifications-cm ConfigMap.
	// +optional
	ArgoCDNotificationsConfigMap map[string]string `json:"argocdNotificationsConfigMap,omitempty"`

	// ArgoCDRBACConfigMap sets keys in the argocd-rbac-cm ConfigMap.
	// +optional
	ArgoCDRBACConfigMap map[string]string `json:"argocdRbacConfigMap,omitempty"`

	// ArgoCDSSHKnownHostsConfigMap sets keys in the
	// argocd-ssh-known-hosts-cm ConfigMap.
	// +optional
	ArgoCDSSHKnownHostsConfigMap map[string]string `json:"argocdSshKnownHostsConfigMap,omitempty"`

	// ArgoCDTLSCertsConfigMap sets keys in the argocd-tls-certs-cm
	// ConfigMap.
	// +optional
	ArgoCDTLSCertsConfigMap map[string]string `json:"argocdTlsCertsConfigMap,omitempty"`

	// ConfigManagementPlugins keys plugin names to their definitions.
	// +optional
	ConfigManagementPlugins map[string]ConfigManagementPlugin `json:"configManagementPlugins,omitempty"`

	// ArgoCDSecretRef references a Secret whose data is sent verbatim
	// as the argocd-secret payload. Typical keys: admin.password,
	// server.secretkey, dex.config, webhook.*.secret, oidc.*.clientSecret.
	// +optional
	ArgoCDSecretRef *xpv1.LocalSecretReference `json:"argocdSecretRef,omitempty"`

	// ArgoCDNotificationsSecretRef references a Secret whose data is
	// sent verbatim as the argocd-notifications-secret payload
	// (SMTP, Slack, webhook tokens).
	// +optional
	ArgoCDNotificationsSecretRef *xpv1.LocalSecretReference `json:"argocdNotificationsSecretRef,omitempty"`

	// ArgoCDImageUpdaterSecretRef references a Secret whose data is
	// sent verbatim as the argocd-image-updater-secret payload
	// (container registry credentials).
	// +optional
	ArgoCDImageUpdaterSecretRef *xpv1.LocalSecretReference `json:"argocdImageUpdaterSecretRef,omitempty"`

	// ApplicationSetSecretRef references a Secret whose data is sent
	// verbatim as the argocd-application-set-secret payload
	// (ApplicationSet plugin credentials).
	// +optional
	ApplicationSetSecretRef *xpv1.LocalSecretReference `json:"applicationSetSecretRef,omitempty"`

	// RepoCredentialSecretRefs registers scoped repository credentials
	// with the Akuity gateway. Each entry's Name (which must match
	// ^repo-[a-z0-9][a-z0-9-]*$) becomes the server-side secret
	// identifier; the pointed-at Secret's data supplies the credential
	// key/value pairs (url, username, password, sshPrivateKey, etc.).
	// The controller applies argocd.argoproj.io/secret-type=repository
	// on submission. The stricter regex (vs bare prefix) rejects
	// whitespace, case drift, and other sneaky inputs that would
	// otherwise parse as a valid prefix but fail downstream.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.all(r, r.name.matches('^repo-[a-z0-9][a-z0-9-]*$'))",message="each repoCredentialSecretRefs[].name must match ^repo-[a-z0-9][a-z0-9-]*$"
	RepoCredentialSecretRefs []NamedLocalSecretReference `json:"repoCredentialSecretRefs,omitempty"`

	// RepoTemplateCredentialSecretRefs registers scoped repository
	// template credentials. Same shape + regex constraint as
	// RepoCredentialSecretRefs; the controller applies
	// argocd.argoproj.io/secret-type=repo-creds on submission.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.all(r, r.name.matches('^repo-[a-z0-9][a-z0-9-]*$'))",message="each repoTemplateCredentialSecretRefs[].name must match ^repo-[a-z0-9][a-z0-9-]*$"
	RepoTemplateCredentialSecretRefs []NamedLocalSecretReference `json:"repoTemplateCredentialSecretRefs,omitempty"`

	// Resources carries raw YAML manifests for declarative ArgoCD
	// child resources. The controller validates each entry's
	// apiVersion/kind (argoproj.io/v1alpha1 Application,
	// ApplicationSet, or AppProject), groups them by kind, and sends
	// them alongside the instance spec on every ApplyInstance call.
	// Write the YAML as an object, not a string; the CRD schema is
	// preserve-unknown-fields so the payload is opaque to kube-
	// apiserver validation. Each entry must still be valid ArgoCD
	// YAML — the Akuity gateway rejects malformed payloads server-
	// side.
	//
	// Additive semantics: removing an entry from this list does NOT
	// delete the corresponding resource on the Akuity platform. The
	// controller intentionally does not enable ApplyInstance's
	// PruneResourceTypes for Applications, ApplicationSets, or
	// AppProjects because out-of-band resources (created via the
	// Akuity UI or another tool) would be collateral damage. To
	// remove a resource, delete it through the Akuity platform UI or
	// API. See PARITY_PLAN.md for the rationale.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Resources []runtime.RawExtension `json:"resources,omitempty"`
}

// ConfigManagementPlugin registers a v2 config-management plugin on the
// argocd-cm ConfigMap. This is a CRD-surface struct distinct from the
// upstream ConfigManagementPlugin Kubernetes resource: upstream's type
// is a Kind with TypeMeta/ObjectMeta, while here we only carry the
// plugin registration fields used by the Akuity Instance API. The
// plugin body itself lives in PluginSpec (a leaf).
type ConfigManagementPlugin struct {
	// Enabled toggles the plugin.
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`
	// Image containing the plugin binary.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`
	// Spec is the plugin definition.
	// +optional
	Spec PluginSpec `json:"spec,omitempty"`
}

// InstanceObservation are the observable fields of an Instance.
type InstanceObservation struct {
	// ID assigned by the Akuity Platform.
	ID string `json:"id,omitempty"`
	// Name of the instance.
	Name string `json:"name,omitempty"`
	// Hostname is the public hostname.
	Hostname string `json:"hostname,omitempty"`
	// ClusterCount is the number of managed clusters attached.
	ClusterCount uint32 `json:"clusterCount,omitempty"`
	// HealthStatus is the instance health.
	HealthStatus ResourceStatusCode `json:"healthStatus,omitempty"`
	// ReconciliationStatus is the instance reconciliation status.
	ReconciliationStatus ResourceStatusCode `json:"reconciliationStatus,omitempty"`
	// OwnerOrganizationName is the Akuity organization owning the
	// instance.
	OwnerOrganizationName string `json:"ownerOrganizationName,omitempty"`
	// ArgoCD is the observed ArgoCD configuration.
	ArgoCD ArgoCDSpec `json:"argocd,omitempty"`
	// ArgoCDConfigMap is the observed argocd-cm ConfigMap.
	ArgoCDConfigMap map[string]string `json:"argocdConfigMap,omitempty"`
	// ArgoCDImageUpdaterConfigMap is the observed
	// argocd-image-updater-config ConfigMap.
	ArgoCDImageUpdaterConfigMap map[string]string `json:"argocdImageUpdaterConfigMap,omitempty"`
	// ArgoCDImageUpdaterSSHConfigMap is the observed
	// argocd-image-updater-ssh-config ConfigMap.
	ArgoCDImageUpdaterSSHConfigMap map[string]string `json:"argocdImageUpdaterSshConfigMap,omitempty"`
	// ArgoCDNotificationsConfigMap is the observed
	// argocd-notifications-cm ConfigMap.
	ArgoCDNotificationsConfigMap map[string]string `json:"argocdNotificationsConfigMap,omitempty"`
	// ArgoCDRBACConfigMap is the observed argocd-rbac-cm ConfigMap.
	ArgoCDRBACConfigMap map[string]string `json:"argocdRbacConfigMap,omitempty"`
	// ArgoCDSSHKnownHostsConfigMap is the observed
	// argocd-ssh-known-hosts-cm ConfigMap.
	ArgoCDSSHKnownHostsConfigMap map[string]string `json:"argocdSshKnownHostsConfigMap,omitempty"`
	// ArgoCDTLSCertsConfigMap is the observed argocd-tls-certs-cm
	// ConfigMap.
	ArgoCDTLSCertsConfigMap map[string]string `json:"argocdTlsCertsConfigMap,omitempty"`
	// ConfigManagementPlugins is the observed plugin set.
	ConfigManagementPlugins map[string]ConfigManagementPlugin `json:"configManagementPlugins,omitempty"`

	// ApplicationsStatus surfaces aggregated counts for the
	// Applications / ApplicationSets / AppProjects managed through
	// spec.forProvider.resources. Compositions and dashboards can use
	// these to gate promotion without querying the Akuity platform
	// UI.
	ApplicationsStatus *ApplicationsStatus `json:"applicationsStatus,omitempty"`

	// SecretHash is the SHA256 of the concatenation of every resolved
	// Secret referenced by spec.forProvider on the most recent Apply.
	// The gateway masks secret contents on Get, so the controller
	// uses this stored digest as the drift signal for Secret rotation:
	// when the hash of the currently-resolved content differs from
	// this value, a re-Apply is scheduled. Stored in status rather
	// than an annotation because managed.Reconciler persists status
	// after Create/Update but does not persist arbitrary metadata.
	SecretHash string `json:"secretHash,omitempty"`
}

// ApplicationsStatus aggregates the per-child health + sync counters
// the Akuity gateway reports on InstanceInfo.ApplicationsStatus.
type ApplicationsStatus struct {
	// ApplicationCount is the total number of Applications observed.
	ApplicationCount uint32 `json:"applicationCount,omitempty"`
	// ResourcesCount is the total number of underlying kube resources
	// owned by those Applications.
	ResourcesCount uint32 `json:"resourcesCount,omitempty"`
	// AppOfAppCount is the subset that are app-of-apps parents.
	AppOfAppCount uint32 `json:"appOfAppCount,omitempty"`
	// SyncInProgressCount is the number of Applications currently
	// reconciling.
	SyncInProgressCount uint32 `json:"syncInProgressCount,omitempty"`
	// WarningCount counts Applications surfacing warnings.
	WarningCount uint32 `json:"warningCount,omitempty"`
	// ErrorCount counts Applications in an errored state.
	ErrorCount uint32 `json:"errorCount,omitempty"`
	// Health is the per-health-status roll-up.
	Health *ApplicationsHealth `json:"health,omitempty"`
	// SyncStatus is the per-sync-status roll-up.
	SyncStatus *ApplicationsSyncStatus `json:"syncStatus,omitempty"`
}

// ApplicationsHealth is the roll-up of Application health states.
type ApplicationsHealth struct {
	HealthyCount     uint32 `json:"healthyCount,omitempty"`
	DegradedCount    uint32 `json:"degradedCount,omitempty"`
	ProgressingCount uint32 `json:"progressingCount,omitempty"`
	UnknownCount     uint32 `json:"unknownCount,omitempty"`
	SuspendedCount   uint32 `json:"suspendedCount,omitempty"`
	MissingCount     uint32 `json:"missingCount,omitempty"`
}

// ApplicationsSyncStatus is the roll-up of Application sync states.
type ApplicationsSyncStatus struct {
	SyncedCount    uint32 `json:"syncedCount,omitempty"`
	OutOfSyncCount uint32 `json:"outOfSyncCount,omitempty"`
	UnknownCount   uint32 `json:"unknownCount,omitempty"`
}

// An InstanceSpec defines the desired state of an Instance.
type InstanceSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              InstanceParameters `json:"forProvider"`
}

// An InstanceStatus represents the observed state of an Instance.
type InstanceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          InstanceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// An Instance is an Akuity ArgoCD managed instance.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,akuity}
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSpec   `json:"spec"`
	Status InstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceList contains a list of Instance.
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instance `json:"items"`
}

// Instance type metadata.
var (
	InstanceKind             = reflect.TypeOf(Instance{}).Name()
	InstanceGroupKind        = schema.GroupKind{Group: Group, Kind: InstanceKind}.String()
	InstanceKindAPIVersion   = InstanceKind + "." + SchemeGroupVersion.String()
	InstanceGroupVersionKind = SchemeGroupVersion.WithKind(InstanceKind)
)

func init() {
	SchemeBuilder.Register(&Instance{}, &InstanceList{})
}
