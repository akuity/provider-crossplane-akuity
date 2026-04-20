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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ClusterSize is the agent-sizing enum accepted by the Akuity API.
// +kubebuilder:validation:Enum=unspecified;small;medium;large;auto
type ClusterSize string

// DirectClusterType selects the kind of directly-attached cluster when
// DirectClusterSpec is set.
type DirectClusterType string

// ClusterParameters are the configurable fields of a Cluster.
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
	// this Cluster containing a kubeconfig that the provider uses to
	// apply agent manifests.
	// +optional
	KubeConfigSecretRef *LocalSecretKeySelector `json:"kubeConfigSecretRef,omitempty"`

	// EnableInClusterKubeConfig makes the provider use its own
	// in-cluster kubeconfig to apply agent manifests. Mutually exclusive
	// with KubeConfigSecretRef.
	// +optional
	EnableInClusterKubeConfig bool `json:"enableInClusterKubeConfig,omitempty"`

	// RemoveAgentResourcesOnDestroy toggles removal of the agent
	// manifests from the managed cluster when the Cluster CR is deleted.
	// Defaults to true when unset.
	// +optional
	RemoveAgentResourcesOnDestroy *bool `json:"removeAgentResourcesOnDestroy,omitempty"`
}

// ClusterData mirrors the upstream ClusterData wire shape with the
// Kustomization field exposed as a YAML string (rather than
// runtime.RawExtension) via the convert glue.
type ClusterData struct {
	// Size selects the agent sizing profile. Defaults to small when
	// unset.
	// +optional
	Size ClusterSize `json:"size,omitempty"`

	// AutoUpgradeDisabled disables automatic agent upgrades.
	// +optional
	AutoUpgradeDisabled *bool `json:"autoUpgradeDisabled,omitempty"`

	// Kustomization is a YAML-encoded Kustomization applied to the
	// agent manifests before they are installed.
	// +optional
	Kustomization string `json:"kustomization,omitempty"`

	// AppReplication toggles ArgoCD application replication to the
	// managed cluster.
	// +optional
	AppReplication *bool `json:"appReplication,omitempty"`

	// TargetVersion pins the agent to a specific version.
	// +optional
	TargetVersion string `json:"targetVersion,omitempty"`

	// RedisTunneling toggles Redis tunneling.
	// +optional
	RedisTunneling *bool `json:"redisTunneling,omitempty"`

	// DirectClusterSpec configures direct-attach cluster credentials.
	// +optional
	DirectClusterSpec *DirectClusterSpec `json:"directClusterSpec,omitempty"`

	// DatadogAnnotationsEnabled toggles emission of Datadog annotations.
	// +optional
	DatadogAnnotationsEnabled *bool `json:"datadogAnnotationsEnabled,omitempty"`

	// EksAddonEnabled toggles the EKS add-on installer path.
	// +optional
	EksAddonEnabled *bool `json:"eksAddonEnabled,omitempty"`

	// ManagedClusterConfig references a cluster-credentials Secret.
	// +optional
	ManagedClusterConfig *ManagedClusterConfig `json:"managedClusterConfig,omitempty"`

	// MaintenanceMode pauses agent reconciliation on the managed
	// cluster.
	// +optional
	MaintenanceMode *bool `json:"maintenanceMode,omitempty"`

	// MultiClusterK8SDashboardEnabled toggles the multi-cluster K8s
	// dashboard integration for this cluster.
	// +optional
	MultiClusterK8SDashboardEnabled *bool `json:"multiClusterK8sDashboardEnabled,omitempty"`

	// AutoscalerConfig configures horizontal autoscaling for agent
	// components.
	// +optional
	AutoscalerConfig *AutoScalerConfig `json:"autoscalerConfig,omitempty"`

	// Project assigns the cluster to an ArgoCD project.
	// +optional
	Project string `json:"project,omitempty"`

	// Compatibility captures platform-compatibility toggles.
	// +optional
	Compatibility *ClusterCompatibility `json:"compatibility,omitempty"`

	// ArgocdNotificationsSettings enables in-cluster notifications.
	// +optional
	ArgocdNotificationsSettings *ClusterArgoCDNotificationsSettings `json:"argocdNotificationsSettings,omitempty"`

	// ServerSideDiffEnabled toggles server-side diff on the agent.
	// +optional
	ServerSideDiffEnabled *bool `json:"serverSideDiffEnabled,omitempty"`

	// MaintenanceModeExpiry scopes MaintenanceMode to an absolute
	// deadline (RFC 3339).
	// +optional
	MaintenanceModeExpiry *metav1.Time `json:"maintenanceModeExpiry,omitempty"`

	// PodInheritMetadata copies selected labels/annotations from the
	// agent pod template to the agent workload.
	// +optional
	PodInheritMetadata *bool `json:"podInheritMetadata,omitempty"`
}

// Resources captures a CPU/memory resource pair.
type Resources struct {
	// Mem is the memory request/limit (Kubernetes resource quantity).
	// +optional
	Mem string `json:"mem,omitempty"`
	// Cpu is the CPU request/limit (Kubernetes resource quantity).
	// +optional
	Cpu string `json:"cpu,omitempty"`
}

// DirectClusterSpec configures a directly-attached (non-agent) cluster.
type DirectClusterSpec struct {
	// ClusterType selects the direct-attach strategy.
	// +optional
	ClusterType DirectClusterType `json:"clusterType,omitempty"`
	// KargoInstanceId pins the owning Kargo instance when ClusterType
	// is Kargo-shaped.
	// +optional
	KargoInstanceId *string `json:"kargoInstanceId,omitempty"`
	// Server is the API server URL for the direct cluster.
	// +optional
	Server *string `json:"server,omitempty"`
	// Organization is a direct-attach organization identifier.
	// +optional
	Organization *string `json:"organization,omitempty"`
	// Token is a bearer-token credential for the direct cluster.
	// +optional
	Token *string `json:"token,omitempty"`
	// CaData is the PEM-encoded CA bundle for the direct cluster.
	// +optional
	CaData *string `json:"caData,omitempty"`
}

// ManagedClusterConfig points the Akuity API at a Secret that holds
// cluster credentials.
type ManagedClusterConfig struct {
	// SecretName is the name of the Secret (in the Akuity managed
	// namespace) storing the credentials.
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// SecretKey is the key within the Secret.
	// +optional
	SecretKey string `json:"secretKey,omitempty"`
}

// AutoScalerConfig describes autoscaling policies for agent components.
type AutoScalerConfig struct {
	// ApplicationController autoscaler config.
	// +optional
	ApplicationController *AppControllerAutoScalingConfig `json:"applicationController,omitempty"`
	// RepoServer autoscaler config.
	// +optional
	RepoServer *RepoServerAutoScalingConfig `json:"repoServer,omitempty"`
}

// AppControllerAutoScalingConfig bounds the application-controller
// resource allocation.
type AppControllerAutoScalingConfig struct {
	// ResourceMinimum requested for the application controller.
	// +optional
	ResourceMinimum *Resources `json:"resourceMinimum,omitempty"`
	// ResourceMaximum allowed for the application controller.
	// +optional
	ResourceMaximum *Resources `json:"resourceMaximum,omitempty"`
}

// RepoServerAutoScalingConfig bounds the repo-server resources and
// replica count.
type RepoServerAutoScalingConfig struct {
	// ResourceMinimum requested for each repo-server replica.
	// +optional
	ResourceMinimum *Resources `json:"resourceMinimum,omitempty"`
	// ResourceMaximum allowed for each repo-server replica.
	// +optional
	ResourceMaximum *Resources `json:"resourceMaximum,omitempty"`
	// ReplicaMaximum caps the number of repo-server replicas.
	// +optional
	ReplicaMaximum int32 `json:"replicaMaximum,omitempty"`
	// ReplicaMinimum floors the number of repo-server replicas.
	// +optional
	ReplicaMinimum int32 `json:"replicaMinimum,omitempty"`
}

// ClusterCompatibility captures platform-compatibility toggles.
type ClusterCompatibility struct {
	// Ipv6Only restricts the agent to IPv6 networking.
	// +optional
	Ipv6Only bool `json:"ipv6Only,omitempty"`
}

// ClusterArgoCDNotificationsSettings controls where notifications run.
type ClusterArgoCDNotificationsSettings struct {
	// InClusterSettings runs notifications from the agent instead of
	// the Akuity control plane.
	// +optional
	InClusterSettings bool `json:"inClusterSettings,omitempty"`
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
type ClusterObservationAgentState struct {
	// Version of the running agent.
	Version string `json:"version,omitempty"`
	// ArgoCdVersion bundled with the running agent.
	ArgoCdVersion string `json:"argoCdVersion,omitempty"`
	// Statuses reports the per-agent health statuses.
	Statuses map[string]ClusterObservationAgentHealthStatus `json:"statuses,omitempty"`
}

// ClusterObservationAgentHealthStatus is a per-agent status snapshot.
type ClusterObservationAgentHealthStatus struct {
	// Code reports the agent's numeric status code.
	Code int32 `json:"code,omitempty"`
	// Message reports the agent's human-readable status message.
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
