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

// ClusterParameters are the configurable fields of a Cluster.
type ClusterParameters struct {
	// The ID of the Akuity ArgoCD instance the cluster belongs to. InstanceID
	// or InstanceRef must be provided.
	InstanceID string `json:"instanceId,omitempty"`
	// The reference to the Akuity ArgoCD instance the cluster belongs to.
	// InstanceID or InstanceRef must be provided.
	InstanceRef NameRef `json:"instanceRef,omitempty"`
	// The name of the cluster. Required.
	Name string `json:"name"`
	// The Kubernetes namespace the Akuity agent should be installed in. Optional.
	Namespace string `json:"namespace,omitempty"`
	// Attributes of the cluster. Optional.
	ClusterSpec generated.ClusterSpec `json:"clusterSpec,omitempty"`
	// Annotations to apply to the cluster custom resource. Optional.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels to apply to the cluster custom resource. Optional.
	Labels map[string]string `json:"labels,omitempty"`
	// A reference to a Kubernetes secret containing a KubeConfig that can be used to connect
	// to the cluster to apply the agent manifests. Optional.
	KubeConfigSecretRef SecretRef `json:"kubeconfigSecretRef,omitempty"`
	// Rather than specifying a reference to a KubeConfig to use to connect to the cluster,
	// we can enable incluster config if the managed cluster is the same cluster the
	// Crossplane is running in. Optional.
	EnableInClusterKubeConfig bool `json:"enableInClusterKubeconfig,omitempty"`
	// Whether or not to remove the Akuity agent Kubernetes resources from the managed cluster
	// when destroying the cluster. Optional. Defaults to true.
	RemoveAgentResourcesOnDestroy bool `json:"removeAgentResourcesOnDestroy,omitempty"`
}

type NameRef struct {
	// The name of the Kubernetes resource being referenced. Required.
	Name string `json:"name"`
}

type SecretRef struct {
	// The name of the Kubernetes secret being referenced. Required.
	Name string `json:"name"`
	// The namespace of the Kubernetes secret being referenced. Required.
	Namespace string `json:"namespace"`
}

// ClusterObservation are the observable fields of a Cluster.
type ClusterObservation struct {
	// The ID of the cluster.
	ID string `json:"id"`
	// The name of the cluster.
	Name string `json:"name"`
	// The description of the cluster.
	Description string `json:"description,omitempty"`
	// The Kubernetes namespace the Akuity agent is installed in.
	Namespace string `json:"namespace,omitempty"`
	// Whether or not the Akuity agent is namespace-scoped.
	NamespaceScoped bool `json:"namespaceScoped,omitempty"`
	// Labels applied to the cluster.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations applied to the cluster.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Whether or not the agent should be autoupgraded when a new version is available.
	AutoUpgradeDisabled bool `json:"autoUpgradeDisabled,omitempty"`
	// Whether or not state replication to the managed cluster is enabled.
	// When enabled, the managed cluster retains core ArgoCD functionality even
	// when unable to connect to the Akuity Platform.
	AppReplication bool `json:"appReplication,omitempty"`
	// The desired version of the agent to run on the cluster.
	TargetVersion string `json:"targetVersion,omitempty"`
	// Whether or not the agent should connect to Redis over a web-socket tunnel
	/// in order to support running the agent behind a HTTPS proxy.
	RedisTunneling bool `json:"redisTunneling,omitempty"`
	// The status of each agent running in the cluster.
	AgentState ClusterObservationAgentState `json:"agentState,omitempty"`
	// The health status of the cluster.
	HealthStatus ClusterObservationStatus `json:"healthStatus,omitempty"`
	// The reconciliation status of the cluster.
	ReconciliationStatus ClusterObservationStatus `json:"reconciliationStatus,omitempty"`
	// A Kustomization to apply to the cluster resource.
	Kustomization string `json:"kustomization,omitempty"`
	// The size of the agent to run on the cluster.
	AgentSize string `json:"agentSize,omitempty"`
}

type ClusterObservationAgentState struct {
	Version       string                                         `json:"version,omitempty"`
	ArgoCdVersion string                                         `json:"argoCdVersion,omitempty"`
	Statuses      map[string]ClusterObservationAgentHealthStatus `json:"statuses,omitempty"`
}

type ClusterObservationAgentHealthStatus struct {
	Code    int32  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type ClusterObservationStatus struct {
	Code    int32  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// A ClusterSpec defines the desired state of a Cluster.
type ClusterSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ClusterParameters `json:"forProvider"`
}

// A ClusterStatus represents the observed state of a Cluster.
type ClusterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ClusterObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Cluster is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,akuity}
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// Cluster type metadata.
var (
	ClusterKind             = reflect.TypeOf(Cluster{}).Name()
	ClusterGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterKind}.String()
	ClusterKindAPIVersion   = ClusterKind + "." + SchemeGroupVersion.String()
	ClusterGroupVersionKind = SchemeGroupVersion.WithKind(ClusterKind)
)

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
