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
// CEL rules:
//   - Exactly one of instanceId or instanceRef must be set.
//   - maintenanceModeExpiry requires maintenanceMode=true (a non-zero
//     expiry without the gate is meaningless).
//   - size=custom requires autoscalerConfig.
//   - autoscalerConfig is only valid for size=auto or size=custom
//     (fixed-size tiers don't autoscale).
//   - kubeconfigSecretRef and enableInClusterKubeconfig are mutually
//     exclusive (both pick the kubeconfig for the agent apply).
//   - removeAgentResourcesOnDestroy requires one of the kubeconfig
//     sources (otherwise the controller has no target to clean).
//
// Rules on nested ClusterData fields live here rather than on
// ClusterData itself because ClusterData is emitted by gencrossplane
// types from the upstream Akuity wire type; marker edits on the leaf
// would be wiped by the next generator sync.
//
// Note: self.data is a non-pointer inline struct on the Go side, so
// kube-apiserver treats it as always-present (it never evaluates to
// absent at admission time). We therefore guard only the
// omitempty-tagged children (size, maintenanceModeExpiry,
// autoscalerConfig) with has() — not data itself.
//
// +kubebuilder:validation:XValidation:rule="has(self.instanceId) != has(self.instanceRef)",message="exactly one of instanceId or instanceRef must be set"
// +kubebuilder:validation:XValidation:rule="!has(self.data.maintenanceModeExpiry) || (has(self.data.maintenanceMode) && self.data.maintenanceMode)",message="maintenanceModeExpiry requires maintenanceMode=true"
// +kubebuilder:validation:XValidation:rule="!has(self.data.size) || self.data.size != 'custom' || has(self.data.autoscalerConfig)",message="size=custom requires autoscalerConfig"
// +kubebuilder:validation:XValidation:rule="!has(self.data.autoscalerConfig) || (has(self.data.size) && (self.data.size == 'auto' || self.data.size == 'custom'))",message="autoscalerConfig only valid for size=auto or size=custom"
// +kubebuilder:validation:XValidation:rule="!(has(self.kubeconfigSecretRef) && has(self.enableInClusterKubeconfig) && self.enableInClusterKubeconfig)",message="kubeconfigSecretRef and enableInClusterKubeconfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.removeAgentResourcesOnDestroy) && self.removeAgentResourcesOnDestroy) || has(self.kubeconfigSecretRef) || (has(self.enableInClusterKubeconfig) && self.enableInClusterKubeconfig)",message="removeAgentResourcesOnDestroy requires kubeconfigSecretRef or enableInClusterKubeconfig"
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

	// KubeConfigSecretRef references a Secret in the same namespace as
	// this Cluster whose "kubeconfig" data key holds a kubeconfig for
	// the managed Kubernetes cluster. When set, the controller uses
	// that kubeconfig to server-side apply the Akuity-generated agent
	// install manifests on Create/Update. Mutually exclusive with
	// EnableInClusterKubeConfig.
	//
	// Cross-namespace kubeconfig Secrets are not supported — the
	// v1alpha2 same-namespace rule applies.
	// +optional
	KubeConfigSecretRef *xpv1.LocalSecretReference `json:"kubeconfigSecretRef,omitempty"`

	// EnableInClusterKubeConfig directs the controller to apply the
	// Akuity agent install manifests using the provider pod's own
	// in-cluster service-account config. Only meaningful when the
	// managed cluster is the same cluster the provider runs in. The
	// provider pod's ServiceAccount must carry permissions sufficient
	// to install the agent (Deployments, CRDs, RBAC, etc.).
	// Mutually exclusive with KubeConfigSecretRef.
	// +optional
	EnableInClusterKubeConfig bool `json:"enableInClusterKubeconfig,omitempty"`

	// RemoveAgentResourcesOnDestroy, when true, causes the controller
	// to delete the Akuity agent resources from the managed cluster on
	// MR deletion — using the same kubeconfig source as install.
	// Only meaningful when one of the kubeconfig fields is set; guarded
	// by a CEL rule on ClusterParameters. Defaults to false.
	// +optional
	RemoveAgentResourcesOnDestroy bool `json:"removeAgentResourcesOnDestroy,omitempty"`
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
	// AgentManifestsHash records the SHA256 of the Akuity-generated
	// agent install manifests that the controller last successfully
	// applied to the managed cluster via KubeConfigSecretRef or
	// EnableInClusterKubeConfig. Empty when inline agent apply is not
	// configured or install has not yet succeeded. Drives re-apply
	// on manifest drift (e.g. when an upgrade on the Akuity side
	// regenerates the bundle).
	AgentManifestsHash string `json:"agentManifestsHash,omitempty"`
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
