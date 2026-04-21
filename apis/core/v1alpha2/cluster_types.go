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

// ClusterParameters are the configurable fields of a Cluster.
//
// +kubebuilder:validation:XValidation:rule="has(self.instanceId) != has(self.instanceRef)",message="exactly one of instanceId or instanceRef must be set"
type ClusterParameters struct {
	// InstanceID references the Akuity ArgoCD Instance the cluster
	// belongs to by its opaque ID. Either InstanceID or InstanceRef must
	// be set.
	// +optional
	InstanceID string `json:"instanceId,omitempty"`

	// InstanceRef references the Akuity ArgoCD Instance the cluster
	// belongs to by name in the same namespace as this Cluster.
	// +optional
	InstanceRef *LocalReference `json:"instanceRef,omitempty"`

	// Name of the cluster in the Akuity Platform. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace in the managed Kubernetes cluster where the Akuity
	// agent should be installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Description is a free-form description of the cluster.
	// +optional
	Description string `json:"description,omitempty"`

	// NamespaceScoped toggles whether the agent is namespace-scoped.
	// +optional
	NamespaceScoped bool `json:"namespaceScoped,omitempty"`

	// Labels applied to the cluster custom resource.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations applied to the cluster custom resource.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Data holds the cluster data configuration.
	// +optional
	Data ClusterData `json:"data,omitempty"`
}

// ClusterObservation are the observable fields of a Cluster.
type ClusterObservation struct {
	// ID assigned by the Akuity Platform.
	ID string `json:"id,omitempty"`
	// Name of the cluster.
	Name string `json:"name,omitempty"`
	// Description of the cluster.
	Description string `json:"description,omitempty"`
	// Namespace in the managed cluster where the agent is installed.
	Namespace string `json:"namespace,omitempty"`
	// NamespaceScoped reports whether the agent is namespace-scoped.
	NamespaceScoped bool `json:"namespaceScoped,omitempty"`
	// Labels applied to the cluster in the Akuity Platform.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations applied to the cluster in the Akuity Platform.
	Annotations map[string]string `json:"annotations,omitempty"`
	// AutoUpgradeDisabled reflects the observed auto-upgrade toggle.
	AutoUpgradeDisabled bool `json:"autoUpgradeDisabled,omitempty"`
	// AppReplication reflects the observed ApplicationReplication
	// toggle.
	AppReplication bool `json:"appReplication,omitempty"`
	// TargetVersion is the agent version currently targeted.
	TargetVersion string `json:"targetVersion,omitempty"`
	// RedisTunneling reflects the observed redis-tunneling toggle.
	RedisTunneling bool `json:"redisTunneling,omitempty"`
	// AgentSize is the observed agent sizing profile.
	AgentSize string `json:"agentSize,omitempty"`
	// Kustomization is the observed kustomization YAML.
	Kustomization string `json:"kustomization,omitempty"`
	// AgentState captures the per-component agent state.
	AgentState ClusterObservationAgentState `json:"agentState,omitempty"`
	// HealthStatus captures the cluster health status code and message.
	HealthStatus ResourceStatusCode `json:"healthStatus,omitempty"`
	// ReconciliationStatus captures the cluster reconciliation status.
	ReconciliationStatus ResourceStatusCode `json:"reconciliationStatus,omitempty"`
}

// ClusterObservationAgentState captures versions and per-agent status.
// Shell-owned: the upstream wire type does not surface a
// per-component agent status breakdown.
type ClusterObservationAgentState struct {
	// Version of the running agent.
	Version string `json:"version,omitempty"`
	// ArgoCdVersion bundled with the running agent.
	ArgoCdVersion string `json:"argoCdVersion,omitempty"`
	// Statuses reports the per-agent health statuses.
	Statuses map[string]ClusterObservationAgentHealthStatus `json:"statuses,omitempty"`
}

// ClusterObservationAgentHealthStatus is a per-agent status snapshot.
// Shell-owned; see ClusterObservationAgentState.
type ClusterObservationAgentHealthStatus struct {
	// Code reports the agent's numeric status code.
	Code int32 `json:"code,omitempty"`
	// Message reports the agent's human-readable status message.
	Message string `json:"message,omitempty"`
}

// A ClusterSpec defines the desired state of a Cluster.
type ClusterSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ClusterParameters `json:"forProvider"`
}

// A ClusterStatus represents the observed state of a Cluster.
type ClusterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ClusterObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Cluster is an Akuity-managed ArgoCD agent cluster.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,akuity}
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster.
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
