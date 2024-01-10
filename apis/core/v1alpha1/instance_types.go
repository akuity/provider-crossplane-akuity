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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	generated "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// InstanceParameters are the configurable fields of a Instance.
type InstanceParameters struct {
	// The name of the instance. Required.
	Name string `json:"name"`

	// The attributes of the instance. Required.
	ArgoCD *generated.ArgoCD `json:"argocd"`

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
	ConfigManagementPlugins map[string]generated.ConfigManagementPlugin `json:"configManagementPlugins,omitempty"`
}

// InstanceObservation are the observable fields of a Instance.
type InstanceObservation struct {
	ID                             string                                      `json:"id"`
	Name                           string                                      `json:"name,omitempty"`
	Hostname                       string                                      `json:"hostname,omitempty"`
	ClusterCount                   uint32                                      `json:"clusterCount,omitempty"`
	HealthStatus                   InstanceObservationStatus                   `json:"healthStatus,omitempty"`
	ReconciliationStatus           InstanceObservationStatus                   `json:"reconciliationStatus,omitempty"`
	OwnerOrganizationName          string                                      `json:"ownerOrganizationName,omitempty"`
	ArgoCD                         generated.ArgoCD                            `json:"argocd"`
	ArgoCDConfigMap                map[string]string                           `json:"argocdConfigMap,omitempty"`
	ArgoCDImageUpdaterConfigMap    map[string]string                           `json:"argocdImageUpdaterConfigMap,omitempty"`
	ArgoCDImageUpdaterSSHConfigMap map[string]string                           `json:"argocdImageUpdaterSshConfigMap,omitempty"`
	ArgoCDNotificationsConfigMap   map[string]string                           `json:"argocdNotificationsConfigMap,omitempty"`
	ArgoCDRBACConfigMap            map[string]string                           `json:"argocdRbacConfigMap,omitempty"`
	ArgoCDSSHKnownHostsConfigMap   map[string]string                           `json:"argocdSshKnownHostsConfigMap,omitempty"`
	ArgoCDTLSCertsConfigMap        map[string]string                           `json:"argocdTlsCertsConfigMap,omitempty"`
	ConfigManagementPlugins        map[string]generated.ConfigManagementPlugin `json:"configManagementPlugins,omitempty"`
}

type InstanceObservationStatus struct {
	Code    int32  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
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
