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

// KargoInstanceParameters are the configurable fields of a Kargo
// instance.
type KargoInstanceParameters struct {
	// Name of the Kargo instance in the Akuity Platform. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Spec describes the Kargo configuration. Required.
	// +kubebuilder:validation:Required
	Spec KargoSpec `json:"spec"`
}

// KargoSpec mirrors the upstream KargoSpec wire type.
type KargoSpec struct {
	// Description is a free-form description of the instance.
	Description string `json:"description"`
	// Version of Kargo to run. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
	// KargoInstanceSpec captures instance-level Kargo configuration.
	// +optional
	KargoInstanceSpec KargoInstanceSpec `json:"kargoInstanceSpec,omitempty"`
	// Fqdn overrides the default Kargo FQDN.
	// +optional
	Fqdn string `json:"fqdn,omitempty"`
	// Subdomain overrides the default Kargo subdomain.
	// +optional
	Subdomain string `json:"subdomain,omitempty"`
	// OidcConfig configures OIDC authentication.
	// +optional
	OidcConfig *KargoOidcConfig `json:"oidcConfig,omitempty"`
}

// KargoInstanceSpec captures instance-level Kargo configuration.
type KargoInstanceSpec struct {
	// BackendIpAllowListEnabled toggles the backend IP allow list.
	// +optional
	BackendIpAllowListEnabled bool `json:"backendIpAllowListEnabled,omitempty"`
	// IpAllowList restricts which IPs can reach the instance.
	// +optional
	IpAllowList []*KargoIPAllowListEntry `json:"ipAllowList,omitempty"`
	// AgentCustomizationDefaults configures default agent
	// customization.
	// +optional
	AgentCustomizationDefaults *KargoAgentCustomization `json:"agentCustomizationDefaults,omitempty"`
	// DefaultShardAgent is the name of the default shard agent used
	// for new projects. Managed by KargoDefaultShardAgent MRs when
	// present.
	// +optional
	DefaultShardAgent string `json:"defaultShardAgent,omitempty"`
	// GlobalCredentialsNs lists namespaces where credentials are
	// shared across projects.
	// +optional
	GlobalCredentialsNs []string `json:"globalCredentialsNs,omitempty"`
	// GlobalServiceAccountNs lists namespaces where service accounts
	// are shared across projects.
	// +optional
	GlobalServiceAccountNs []string `json:"globalServiceAccountNs,omitempty"`
	// AkuityIntelligence configures the Akuity Intelligence
	// integration.
	// +optional
	AkuityIntelligence *AkuityIntelligence `json:"akuityIntelligence,omitempty"`
	// GcConfig configures the garbage collector.
	// +optional
	GcConfig *GarbageCollectorConfig `json:"gcConfig,omitempty"`
	// PromoControllerEnabled toggles the promotion controller.
	// +optional
	PromoControllerEnabled bool `json:"promoControllerEnabled,omitempty"`
	// Secrets configures cross-cluster secret management for Kargo.
	// +optional
	Secrets SecretsManagementConfig `json:"secrets,omitempty"`
	// ArgocdUi configures the Kargo-side ArgoCD UI integration.
	// +optional
	ArgocdUi *KargoArgoCDUIConfig `json:"argocdUi,omitempty"`
}

// KargoArgoCDUIConfig configures the Kargo-side ArgoCD UI integration.
type KargoArgoCDUIConfig struct {
	// IdpGroupsMapping toggles IdP-group mapping in the ArgoCD UI.
	// +optional
	IdpGroupsMapping bool `json:"idpGroupsMapping,omitempty"`
}

// KargoOidcConfig configures OIDC authentication for a Kargo instance.
type KargoOidcConfig struct {
	// Enabled toggles OIDC authentication.
	Enabled bool `json:"enabled"`
	// DexEnabled toggles the Dex OIDC proxy.
	DexEnabled bool `json:"dexEnabled"`
	// DexConfig is the Dex configuration (YAML).
	DexConfig string `json:"dexConfig"`
	// DexConfigSecret maps keys into Dex config values.
	DexConfigSecret map[string]Value `json:"dexConfigSecret"`
	// IssuerURL of the OIDC provider.
	IssuerURL string `json:"issuerUrl"`
	// ClientID used by Kargo.
	ClientID string `json:"clientId"`
	// CliClientID used by the Kargo CLI.
	CliClientID string `json:"cliClientId"`
	// AdminAccount predefined admin account.
	AdminAccount KargoPredefinedAccountData `json:"adminAccount"`
	// ViewerAccount predefined viewer account.
	ViewerAccount KargoPredefinedAccountData `json:"viewerAccount"`
	// AdditionalScopes requested from the OIDC provider.
	AdditionalScopes []string `json:"additionalScopes"`
	// UserAccount predefined user account.
	UserAccount KargoPredefinedAccountData `json:"userAccount"`
	// ProjectCreatorAccount predefined project-creator account.
	ProjectCreatorAccount KargoPredefinedAccountData `json:"projectCreatorAccount"`
}

// Value is a value reference inside DexConfigSecret.
type Value struct {
	// Value is the literal value.
	// +optional
	Value *string `json:"value,omitempty"`
}

// KargoPredefinedAccountData declares claims for a predefined Kargo
// account.
type KargoPredefinedAccountData struct {
	// Claims maps a claim name to its accepted values.
	// +optional
	Claims map[string]KargoPredefinedAccountClaimValue `json:"claims,omitempty"`
}

// KargoPredefinedAccountClaimValue lists values that satisfy a
// predefined claim.
type KargoPredefinedAccountClaimValue struct {
	// Values that satisfy the claim.
	Values []string `json:"values"`
}

// KargoIPAllowListEntry grants Kargo access for an IP/CIDR.
type KargoIPAllowListEntry struct {
	// Ip is the IP address or CIDR.
	// +optional
	Ip string `json:"ip,omitempty"`
	// Description is free-form context.
	// +optional
	Description string `json:"description,omitempty"`
}

// KargoAgentCustomization configures default agent customization.
type KargoAgentCustomization struct {
	// AutoUpgradeDisabled disables automatic agent upgrades.
	// +optional
	AutoUpgradeDisabled bool `json:"autoUpgradeDisabled,omitempty"`
	// Kustomization YAML applied to agent manifests.
	// +optional
	Kustomization string `json:"kustomization,omitempty"`
}

// AkuityIntelligence configures the Akuity Intelligence features for a
// Kargo instance.
type AkuityIntelligence struct {
	// AiSupportEngineerEnabled toggles the AI support engineer persona.
	// +optional
	AiSupportEngineerEnabled bool `json:"aiSupportEngineerEnabled,omitempty"`
	// Enabled toggles the integration.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// AllowedUsernames grants access to named users.
	// +optional
	AllowedUsernames []string `json:"allowedUsernames,omitempty"`
	// AllowedGroups grants access to named groups.
	// +optional
	AllowedGroups []string `json:"allowedGroups,omitempty"`
	// ModelVersion pins the AI model version.
	// +optional
	ModelVersion string `json:"modelVersion,omitempty"`
}

// GarbageCollectorConfig bounds Kargo freight/promotion retention.
type GarbageCollectorConfig struct {
	// MaxRetainedFreight caps retained freight per warehouse.
	// +optional
	MaxRetainedFreight uint32 `json:"maxRetainedFreight,omitempty"`
	// MaxRetainedPromotions caps retained promotions per stage.
	// +optional
	MaxRetainedPromotions uint32 `json:"maxRetainedPromotions,omitempty"`
	// MinFreightDeletionAge floors freight retention in seconds.
	// +optional
	MinFreightDeletionAge uint32 `json:"minFreightDeletionAge,omitempty"`
	// MinPromotionDeletionAge floors promotion retention in seconds.
	// +optional
	MinPromotionDeletionAge uint32 `json:"minPromotionDeletionAge,omitempty"`
}

// KargoInstanceObservation are the observable fields of a Kargo
// instance.
type KargoInstanceObservation struct {
	// ID assigned by the Akuity Platform.
	ID string `json:"id,omitempty"`
	// Name of the instance.
	Name string `json:"name,omitempty"`
	// Hostname is the public hostname.
	Hostname string `json:"hostname,omitempty"`
	// HealthStatus is the instance health.
	HealthStatus ResourceStatusCode `json:"healthStatus,omitempty"`
	// ReconciliationStatus is the instance reconciliation status.
	ReconciliationStatus ResourceStatusCode `json:"reconciliationStatus,omitempty"`
	// OwnerOrganizationName is the Akuity organization owning the
	// instance.
	OwnerOrganizationName string `json:"ownerOrganizationName,omitempty"`
}

// A KargoInstanceSpecResource defines the desired state of a Kargo
// instance.
type KargoInstanceSpecResource struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       KargoInstanceParameters `json:"forProvider"`
}

// A KargoInstanceStatus represents the observed state of a Kargo
// instance.
type KargoInstanceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          KargoInstanceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A KargoInstance is an Akuity-managed Kargo control-plane instance.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,akuity}
type KargoInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KargoInstanceSpecResource `json:"spec"`
	Status KargoInstanceStatus       `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KargoInstanceList contains a list of KargoInstance.
type KargoInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KargoInstance `json:"items"`
}

// KargoInstance type metadata.
var (
	KargoInstanceKind             = reflect.TypeOf(KargoInstance{}).Name()
	KargoInstanceGroupKind        = schema.GroupKind{Group: Group, Kind: KargoInstanceKind}.String()
	KargoInstanceKindAPIVersion   = KargoInstanceKind + "." + SchemeGroupVersion.String()
	KargoInstanceGroupVersionKind = SchemeGroupVersion.WithKind(KargoInstanceKind)
)

func init() {
	SchemeBuilder.Register(&KargoInstance{}, &KargoInstanceList{})
}
