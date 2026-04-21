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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// InstanceParameters are the configurable fields of an ArgoCD Instance.
type InstanceParameters struct {
	// Name of the instance in the Akuity Platform. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ArgoCD contains the ArgoCD configuration. Required.
	// +kubebuilder:validation:Required
	ArgoCD *ArgoCD `json:"argocd"`

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
}

// ArgoCD is the v1alpha2 wrapper around the upstream ArgoCD wire type.
// The upstream ArgoCD carries Kubernetes metadata we don't need on the
// CRD surface; this shell-level wrapper reduces it to the Spec.
type ArgoCD struct {
	Spec ArgoCDSpec `json:"spec,omitempty"`
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
	ArgoCD ArgoCD `json:"argocd,omitempty"`
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
