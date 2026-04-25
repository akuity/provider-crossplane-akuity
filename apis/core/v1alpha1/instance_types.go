/*
Copyright 2022 The Crossplane Authors.

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

// InstanceParameters are the configurable fields of a Instance.
//
// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="name is immutable"
type InstanceParameters struct {
	// The name of the instance. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// The attributes of the instance. Required.
	ArgoCD *crossplanetypes.ArgoCD `json:"argocd"`

	// Workspace is the Akuity workspace this Argo CD instance belongs to. The
	// preferred value is the workspace ID because workspace-scoped gateway calls
	// are routed by ID. A workspace name is also accepted and resolved before
	// gateway calls. When omitted, the organization default workspace is used on
	// create. The canonical workspace ID is reported in
	// status.atProvider.workspace.
	// +optional
	Workspace string `json:"workspace,omitempty"`

	// Used to specify the options in the argocd-cm ConfigMap. Optional.
	// Refer to the example in github.com/akuity/provider-crossplane-akuity/examples/instance.yaml
	// for required keys when using this option.
	ArgoCDConfigMap map[string]string `json:"argocdConfigMap,omitempty"`

	// Used to specify the options in the argocd-image-updater-config ConfigMap. Optional.
	ArgoCDImageUpdaterConfigMap map[string]string `json:"argocdImageUpdaterConfigMap,omitempty"`

	// Used to specify the options in the argocd-image-updater-ssh-config ConfigMap. Optional.
	ArgoCDImageUpdaterSSHConfigMap map[string]string `json:"argocdImageUpdaterSshConfigMap,omitempty"`

	// Used to specify the options in the argocd-notifications-cm ConfigMap. Optional.
	ArgoCDNotificationsConfigMap map[string]string `json:"argocdNotificationsConfigMap,omitempty"`

	// Used to specify the options in the argocd-rbac-cm ConfigMap. Optional.
	ArgoCDRBACConfigMap map[string]string `json:"argocdRbacConfigMap,omitempty"`

	// Used to specify the options in the argocd-ssh-known-hosts-cm ConfigMap. Optional.
	// Refer to the example in github.com/akuity/provider-crossplane-akuity/examples/instance.yaml
	// for required entries when using this option.
	ArgoCDSSHKnownHostsConfigMap map[string]string `json:"argocdSshKnownHostsConfigMap,omitempty"`

	// Used to specify the options in the argocd-tls-certs-cm ConfigMap. Optional.
	ArgoCDTLSCertsConfigMap map[string]string `json:"argocdTlsCertsConfigMap,omitempty"`

	// Used to specify config management plugins. The key of the map entry is the name of the
	// plugin. The value is the definition of the Config Management Plugin (v2). Optional.
	ConfigManagementPlugins map[string]crossplanetypes.ConfigManagementPlugin `json:"configManagementPlugins,omitempty"`

	// ArgoCDSecretRef references a namespaced Secret whose data is sent
	// verbatim as the argocd-secret payload (admin.password,
	// server.secretkey, dex.config, webhook.*.secret,
	// oidc.*.clientSecret). Removing this ref stops applying the
	// platform-side Secret, but does not delete it from the Akuity
	// platform.
	// +optional
	// +kubebuilder:validation:XValidation:rule="has(self.name) && size(self.name) > 0 && has(self.namespace) && size(self.namespace) > 0",message="argocdSecretRef.name and argocdSecretRef.namespace are required"
	ArgoCDSecretRef *xpv1.SecretReference `json:"argocdSecretRef,omitempty"`

	// ArgoCDNotificationsSecretRef references a namespaced Secret
	// whose data is sent verbatim as the argocd-notifications-secret
	// payload (SMTP, Slack, webhook tokens). Removing this ref stops
	// applying the platform-side Secret, but does not delete it from
	// the Akuity platform.
	// +optional
	// +kubebuilder:validation:XValidation:rule="has(self.name) && size(self.name) > 0 && has(self.namespace) && size(self.namespace) > 0",message="argocdNotificationsSecretRef.name and argocdNotificationsSecretRef.namespace are required"
	ArgoCDNotificationsSecretRef *xpv1.SecretReference `json:"argocdNotificationsSecretRef,omitempty"`

	// ArgoCDImageUpdaterSecretRef references a namespaced Secret whose
	// data is sent verbatim as the argocd-image-updater-secret payload
	// (container registry credentials). Removing this ref stops
	// applying the platform-side Secret, but does not delete it from
	// the Akuity platform.
	// +optional
	// +kubebuilder:validation:XValidation:rule="has(self.name) && size(self.name) > 0 && has(self.namespace) && size(self.namespace) > 0",message="argocdImageUpdaterSecretRef.name and argocdImageUpdaterSecretRef.namespace are required"
	ArgoCDImageUpdaterSecretRef *xpv1.SecretReference `json:"argocdImageUpdaterSecretRef,omitempty"`

	// ApplicationSetSecretRef references a namespaced Secret whose data
	// is sent verbatim as the argocd-application-set-secret payload
	// (ApplicationSet plugin credentials). Removing this ref stops
	// applying the platform-side Secret, but does not delete it from
	// the Akuity platform.
	// +optional
	// +kubebuilder:validation:XValidation:rule="has(self.name) && size(self.name) > 0 && has(self.namespace) && size(self.namespace) > 0",message="applicationSetSecretRef.name and applicationSetSecretRef.namespace are required"
	ApplicationSetSecretRef *xpv1.SecretReference `json:"applicationSetSecretRef,omitempty"`

	// RepoCredentialSecretRefs registers scoped repository credentials
	// with the Akuity gateway. Each entry's Name (which must match
	// ^repo-[a-z0-9][a-z0-9-]*$) becomes the server-side secret
	// identifier; the pointed-at Secret's data supplies the credential
	// key/value pairs (url, username, password, sshPrivateKey, etc.).
	// If an entry omits Name, the controller uses SecretRef.Name as
	// the server-side secret identifier.
	// Removing an entry stops applying that platform-side credential,
	// but does not delete it from the Akuity platform.
	// +optional
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(r, !has(r.name) || r.name.matches('^repo-[a-z0-9][a-z0-9-]*$'))",message="each repoCredentialSecretRefs[] name must match ^repo-[a-z0-9][a-z0-9-]*$ when set"
	RepoCredentialSecretRefs []NamedSecretReference `json:"repoCredentialSecretRefs,omitempty"`

	// RepoTemplateCredentialSecretRefs registers scoped repository
	// template credentials. Same shape + regex constraint as
	// RepoCredentialSecretRefs.
	// +optional
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(r, !has(r.name) || r.name.matches('^repo-[a-z0-9][a-z0-9-]*$'))",message="each repoTemplateCredentialSecretRefs[] name must match ^repo-[a-z0-9][a-z0-9-]*$ when set"
	RepoTemplateCredentialSecretRefs []NamedSecretReference `json:"repoTemplateCredentialSecretRefs,omitempty"`

	// Resources carries raw YAML manifests for declarative ArgoCD
	// child resources (Application, ApplicationSet, AppProject).
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Resources []runtime.RawExtension `json:"resources,omitempty"`
}

// InstanceObservation are the observable fields of a Instance.
type InstanceObservation struct {
	ID                             string                                            `json:"id"`
	Name                           string                                            `json:"name,omitempty"`
	Hostname                       string                                            `json:"hostname,omitempty"`
	ClusterCount                   uint32                                            `json:"clusterCount,omitempty"`
	HealthStatus                   ResourceStatusCode                                `json:"healthStatus,omitempty"`
	ReconciliationStatus           ResourceStatusCode                                `json:"reconciliationStatus,omitempty"`
	OwnerOrganizationName          string                                            `json:"ownerOrganizationName,omitempty"`
	ArgoCD                         crossplanetypes.ArgoCD                            `json:"argocd"`
	Workspace                      string                                            `json:"workspace,omitempty"`
	ArgoCDConfigMap                map[string]string                                 `json:"argocdConfigMap,omitempty"`
	ArgoCDImageUpdaterConfigMap    map[string]string                                 `json:"argocdImageUpdaterConfigMap,omitempty"`
	ArgoCDImageUpdaterSSHConfigMap map[string]string                                 `json:"argocdImageUpdaterSshConfigMap,omitempty"`
	ArgoCDNotificationsConfigMap   map[string]string                                 `json:"argocdNotificationsConfigMap,omitempty"`
	ArgoCDRBACConfigMap            map[string]string                                 `json:"argocdRbacConfigMap,omitempty"`
	ArgoCDSSHKnownHostsConfigMap   map[string]string                                 `json:"argocdSshKnownHostsConfigMap,omitempty"`
	ArgoCDTLSCertsConfigMap        map[string]string                                 `json:"argocdTlsCertsConfigMap,omitempty"`
	ConfigManagementPlugins        map[string]crossplanetypes.ConfigManagementPlugin `json:"configManagementPlugins,omitempty"`

	// SecretHash is the SHA256 of the concatenation of every resolved
	// Secret referenced by spec.forProvider on the most recent Apply.
	// Used as the drift signal for Secret rotation.
	// +optional
	SecretHash string `json:"secretHash,omitempty"`
}

// A InstanceSpec defines the desired state of a Instance.
type InstanceSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       InstanceParameters `json:"forProvider"`
}

// A InstanceStatus represents the observed state of a Instance.
type InstanceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          InstanceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Instance is an Akuity ArgoCD Instance API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akuity}
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSpec   `json:"spec"`
	Status InstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceList contains a list of Instance
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
