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
	"k8s.io/apimachinery/pkg/runtime/schema"

	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// ClusterParameters are the configurable fields of a Cluster.
//
// Stored legacy Clusters may carry both instanceId and instanceRef.
// A strict XOR rule would reject updates to those CRs because CRD
// validation ratcheting cannot decompose a cross-field rule. The rule
// therefore requires at least one value; the controller resolves
// instanceRef first and falls back to instanceId, so both-set state is
// well-defined.
//
// The immutability rule uses has() guards because k8s CEL raises "no
// such key" when reading an absent omitempty string. This allows the
// controller's first-time instanceId stamp after resolving instanceRef,
// then requires stable values on subsequent updates.
//
// +kubebuilder:validation:XValidation:rule="has(self.instanceId) || has(self.instanceRef)",message="instanceId or instanceRef must be set"
// +kubebuilder:validation:XValidation:rule="(!has(oldSelf.instanceId) || (has(self.instanceId) && self.instanceId == oldSelf.instanceId)) && (!has(oldSelf.instanceRef) || (has(self.instanceRef) && self.instanceRef.name == oldSelf.instanceRef.name))",message="instanceId/instanceRef are immutable"
// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="name is immutable"
type ClusterParameters struct {
	// InstanceID is the Akuity Argo CD instance ID this cluster belongs
	// to. At least one of InstanceID or InstanceRef must be set; when
	// both are present, InstanceID is used.
	// +optional
	InstanceID string `json:"instanceId,omitempty"`
	// InstanceRef references the Akuity Argo CD instance this cluster
	// belongs to. At least one of InstanceID or InstanceRef must be set.
	// +optional
	InstanceRef *LocalReference `json:"instanceRef,omitempty"`
	// Name is the Akuity cluster name. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace is where the Akuity agent is installed.
	Namespace string `json:"namespace,omitempty"`
	// ClusterSpec contains the cluster configuration sent to the Akuity platform.
	ClusterSpec crossplanetypes.ClusterSpec `json:"clusterSpec,omitempty"`
	// Annotations applied to the cluster custom resource.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels applied to the cluster custom resource.
	Labels map[string]string `json:"labels,omitempty"`
	// KubeConfigSecretRef references a Secret containing a kubeconfig
	// used to apply agent manifests to the managed cluster.
	KubeConfigSecretRef xpv1.SecretReference `json:"kubeconfigSecretRef,omitempty"`
	// EnableInClusterKubeConfig uses the provider pod's in-cluster
	// configuration when the managed cluster is the provider cluster.
	EnableInClusterKubeConfig bool `json:"enableInClusterKubeconfig,omitempty"`
	// RemoveAgentResourcesOnDestroy removes Akuity agent Kubernetes
	// resources from the managed cluster during deletion. Defaults to true.
	RemoveAgentResourcesOnDestroy bool `json:"removeAgentResourcesOnDestroy,omitempty"`
}

// ClusterObservation contains the observable fields of a Cluster.
type ClusterObservation struct {
	// The ID of the cluster.
	ID string `json:"id"`
	// The name of the cluster.
	Name string `json:"name"`
	// The description of the cluster.
	//
	// Deprecated: read via ClusterSpec.Description. Retained for
	// backward compatibility with consumers that grew up on the
	// flat-field observation; will be removed in the next API
	// version bump.
	Description string `json:"description,omitempty"`
	// The Kubernetes namespace the Akuity agent is installed in.
	Namespace string `json:"namespace,omitempty"`
	// Whether the Akuity agent is namespace-scoped.
	//
	// Deprecated: read via ClusterSpec.NamespaceScoped. See
	// Description for the deprecation rationale.
	NamespaceScoped bool `json:"namespaceScoped,omitempty"`
	// Labels applied to the cluster.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations applied to the cluster.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Whether agent auto-upgrade is disabled when a new version is available.
	//
	// Deprecated: read via ClusterSpec.Data.AutoUpgradeDisabled.
	AutoUpgradeDisabled bool `json:"autoUpgradeDisabled,omitempty"`
	// Whether state replication to the managed cluster is enabled.
	// When enabled, the managed cluster retains core ArgoCD functionality even
	// when unable to connect to the Akuity Platform.
	//
	// Deprecated: read via ClusterSpec.Data.AppReplication.
	AppReplication bool `json:"appReplication,omitempty"`
	// The desired version of the agent to run on the cluster.
	//
	// Deprecated: read via ClusterSpec.Data.TargetVersion.
	TargetVersion string `json:"targetVersion,omitempty"`
	// Whether the agent connects to Redis over a websocket tunnel to
	// support running behind an HTTPS proxy.
	//
	// Deprecated: read via ClusterSpec.Data.RedisTunneling.
	RedisTunneling bool `json:"redisTunneling,omitempty"`
	// The status of each agent running in the cluster.
	AgentState ClusterObservationAgentState `json:"agentState,omitempty"`
	// The health status of the cluster.
	HealthStatus ResourceStatusCode `json:"healthStatus,omitempty"`
	// The reconciliation status of the cluster.
	ReconciliationStatus ResourceStatusCode `json:"reconciliationStatus,omitempty"`
	// A Kustomization to apply to the cluster resource.
	//
	// Deprecated: read via ClusterSpec.Data.Kustomization.
	Kustomization string `json:"kustomization,omitempty"`
	// The size of the agent to run on the cluster.
	//
	// Deprecated: read via ClusterSpec.Data.Size.
	AgentSize string `json:"agentSize,omitempty"`

	// ClusterSpec mirrors the desired payload observed on the most
	// recent reconcile (description + namespaceScoped + data),
	// matching the shape of spec.forProvider.clusterSpec. Provides
	// symmetry with Instance's atProvider.argocd mirror: users can
	// read the observed payload as one nested block instead of
	// grepping a dozen flat fields.
	// +optional
	ClusterSpec crossplanetypes.ClusterSpec `json:"clusterSpec,omitempty"`
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

// A Cluster is an Akuity Argo CD cluster registration.
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

// ClusterList contains a list of Cluster objects.
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
