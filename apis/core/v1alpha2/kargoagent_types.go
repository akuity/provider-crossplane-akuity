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

// KargoAgentSize is the agent-sizing enum accepted by the Kargo API.
// +kubebuilder:validation:Enum=unspecified;small;medium;large;auto
type KargoAgentSize string

// KargoAgentParameters are the configurable fields of a KargoAgent.
type KargoAgentParameters struct {
	// KargoInstanceID references the owning Kargo instance by ID.
	// Either KargoInstanceID or KargoInstanceRef must be set.
	// +optional
	KargoInstanceID string `json:"kargoInstanceId,omitempty"`

	// KargoInstanceRef references the owning Kargo instance by name in
	// the same namespace as this KargoAgent.
	// +optional
	KargoInstanceRef *LocalReference `json:"kargoInstanceRef,omitempty"`

	// Workspace is the Kargo project/workspace the agent is bound to.
	// +optional
	Workspace string `json:"workspace,omitempty"`

	// Name of the agent. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace in the managed cluster where the agent is installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Labels applied to the agent.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations applied to the agent.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Spec is the KargoAgent configuration.
	// +optional
	Spec *KargoAgentSpec `json:"spec,omitempty"`
}

// KargoAgentSpec is the Kargo-agent wire-level spec.
type KargoAgentSpec struct {
	// Description is a free-form description of the agent.
	// +optional
	Description string `json:"description,omitempty"`
	// Data carries the agent configuration.
	// +optional
	Data KargoAgentData `json:"data,omitempty"`
}

// KargoAgentData configures a Kargo agent.
type KargoAgentData struct {
	// Size selects the agent sizing profile.
	// +optional
	Size KargoAgentSize `json:"size,omitempty"`
	// AutoUpgradeDisabled disables automatic agent upgrades.
	// +optional
	AutoUpgradeDisabled *bool `json:"autoUpgradeDisabled,omitempty"`
	// TargetVersion pins the agent to a specific version.
	// +optional
	TargetVersion string `json:"targetVersion,omitempty"`
	// Kustomization YAML applied to agent manifests.
	// +optional
	Kustomization string `json:"kustomization,omitempty"`
	// RemoteArgocd is the URL of a remote ArgoCD instance the agent
	// should talk to.
	// +optional
	RemoteArgocd string `json:"remoteArgocd,omitempty"`
	// AkuityManaged marks the agent as Akuity-managed.
	// +optional
	AkuityManaged bool `json:"akuityManaged,omitempty"`
	// ArgocdNamespace is the ArgoCD namespace the agent observes.
	// +optional
	ArgocdNamespace string `json:"argocdNamespace,omitempty"`
	// SelfManagedArgocdUrl is the URL of a self-managed ArgoCD
	// instance bound to this agent.
	// +optional
	SelfManagedArgocdUrl string `json:"selfManagedArgocdUrl,omitempty"`
	// AllowedJobSa lists service-account names allowed to run jobs for
	// this agent.
	// +optional
	AllowedJobSa []string `json:"allowedJobSa,omitempty"`
	// MaintenanceMode pauses reconciliation.
	// +optional
	MaintenanceMode *bool `json:"maintenanceMode,omitempty"`
	// MaintenanceModeExpiry scopes MaintenanceMode to an absolute
	// deadline.
	// +optional
	MaintenanceModeExpiry *metav1.Time `json:"maintenanceModeExpiry,omitempty"`
	// PodInheritMetadata copies selected labels/annotations from the
	// agent pod template to the agent workload.
	// +optional
	PodInheritMetadata *bool `json:"podInheritMetadata,omitempty"`
	// AutoscalerConfig configures Kargo-controller autoscaling.
	// +optional
	AutoscalerConfig *KargoAutoscalerConfig `json:"autoscalerConfig,omitempty"`
}

// KargoResources captures a CPU/memory resource pair for Kargo.
type KargoResources struct {
	// Mem is the memory request/limit.
	// +optional
	Mem string `json:"mem,omitempty"`
	// Cpu is the CPU request/limit.
	// +optional
	Cpu string `json:"cpu,omitempty"`
}

// KargoControllerAutoScalingConfig bounds Kargo-controller resources.
type KargoControllerAutoScalingConfig struct {
	// ResourceMinimum requested for the controller.
	// +optional
	ResourceMinimum *KargoResources `json:"resourceMinimum,omitempty"`
	// ResourceMaximum allowed for the controller.
	// +optional
	ResourceMaximum *KargoResources `json:"resourceMaximum,omitempty"`
}

// KargoAutoscalerConfig configures Kargo-controller autoscaling.
type KargoAutoscalerConfig struct {
	// KargoController autoscaler config.
	// +optional
	KargoController *KargoControllerAutoScalingConfig `json:"kargoController,omitempty"`
}

// KargoAgentObservation are the observable fields of a KargoAgent.
type KargoAgentObservation struct {
	// ID assigned by the Akuity Platform.
	ID string `json:"id,omitempty"`
	// Workspace the agent is bound to.
	Workspace string `json:"workspace,omitempty"`
	// HealthStatus is the agent health.
	HealthStatus ResourceStatusCode `json:"healthStatus,omitempty"`
	// ReconciliationStatus is the agent reconciliation status.
	ReconciliationStatus ResourceStatusCode `json:"reconciliationStatus,omitempty"`
}

// A KargoAgentResourceSpec defines the desired state of a KargoAgent.
type KargoAgentResourceSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              KargoAgentParameters `json:"forProvider"`
}

// A KargoAgentStatus represents the observed state of a KargoAgent.
type KargoAgentStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          KargoAgentObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A KargoAgent is a Kargo agent installed in a managed Kubernetes
// cluster.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,akuity}
type KargoAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KargoAgentResourceSpec `json:"spec"`
	Status KargoAgentStatus       `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KargoAgentList contains a list of KargoAgent.
type KargoAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KargoAgent `json:"items"`
}

// KargoAgent type metadata.
var (
	KargoAgentKind             = reflect.TypeOf(KargoAgent{}).Name()
	KargoAgentGroupKind        = schema.GroupKind{Group: Group, Kind: KargoAgentKind}.String()
	KargoAgentKindAPIVersion   = KargoAgentKind + "." + SchemeGroupVersion.String()
	KargoAgentGroupVersionKind = SchemeGroupVersion.WithKind(KargoAgentKind)
)

func init() {
	SchemeBuilder.Register(&KargoAgent{}, &KargoAgentList{})
}
