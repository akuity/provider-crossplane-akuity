// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2026 Akuity, Inc.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// +kubebuilder:object:generate=true
type Kargo struct {
	Spec KargoSpec `json:"spec,omitempty"`
}

// KargoOidcConfig is emitted from the template (not from codegenStructs)
// because it carries provider-specific overlays the Akuity Crossplane
// provider needs but the upstream API source type does not define:
//
//   - DexConfigSecretRef: a same-namespace Secret reference resolved at
//     reconcile time into the wire-shape DexConfigSecret map. Plaintext
//     dex credentials never live on the managed resource spec.
//   - // +optional markers on every field so controller-gen drops them
//     from the CRD's `required` array; the Akuity gateway accepts
//     ref-only or inline OIDC configurations.
//
// +kubebuilder:object:generate=true
type KargoOidcConfig struct {
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// +optional
	DexEnabled *bool `json:"dexEnabled,omitempty"`
	// +optional
	DexConfig string `json:"dexConfig,omitempty"`
	// +optional
	DexConfigSecret map[string]Value `json:"dexConfigSecret,omitempty"`
	// DexConfigSecretRef references a Secret whose data is resolved at
	// reconcile time by the Crossplane provider and forwarded to the
	// Akuity gateway as the wire-shape DexConfigSecret map. Mutually
	// exclusive with the inline DexConfigSecret above.
	// +optional
	DexConfigSecretRef *corev1.LocalObjectReference `json:"dexConfigSecretRef,omitempty"`
	// +optional
	IssuerURL string `json:"issuerUrl,omitempty"`
	// +optional
	ClientID string `json:"clientId,omitempty"`
	// +optional
	CliClientID string `json:"cliClientId,omitempty"`
	// +optional
	AdminAccount KargoPredefinedAccountData `json:"adminAccount,omitempty"`
	// +optional
	ViewerAccount KargoPredefinedAccountData `json:"viewerAccount,omitempty"`
	// +optional
	AdditionalScopes []string `json:"additionalScopes,omitempty"`
	// +optional
	UserAccount KargoPredefinedAccountData `json:"userAccount,omitempty"`
	// +optional
	ProjectCreatorAccount KargoPredefinedAccountData `json:"projectCreatorAccount,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoSpec struct {
	Description       string            `json:"description"`
	Version           string            `json:"version"`
	KargoInstanceSpec KargoInstanceSpec `json:"kargoInstanceSpec,omitempty"`
	Fqdn              string            `json:"fqdn,omitempty"`
	Subdomain         string            `json:"subdomain,omitempty"`
	OidcConfig        *KargoOidcConfig  `json:"oidcConfig,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoPredefinedAccountClaimValue struct {
	Values []string `json:"values"`
}

// +kubebuilder:object:generate=true
type KargoPredefinedAccountData struct {
	Claims map[string]KargoPredefinedAccountClaimValue `json:"claims,omitempty"`
}

// +kubebuilder:object:generate=true
type Value struct {
	Value *string `json:"value,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoIPAllowListEntry struct {
	Ip          string `json:"ip,omitempty"`
	Description string `json:"description,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoAgentCustomization struct {
	AutoUpgradeDisabled *bool  `json:"autoUpgradeDisabled,omitempty"`
	Kustomization       string `json:"kustomization,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoInstanceSpec struct {
	BackendIpAllowListEnabled  *bool                    `json:"backendIpAllowListEnabled,omitempty"`
	IpAllowList                []*KargoIPAllowListEntry `json:"ipAllowList,omitempty"`
	AgentCustomizationDefaults *KargoAgentCustomization `json:"agentCustomizationDefaults,omitempty"`
	DefaultShardAgent          string                   `json:"defaultShardAgent,omitempty"`
	GlobalCredentialsNs        []string                 `json:"globalCredentialsNs,omitempty"`
	GlobalServiceAccountNs     []string                 `json:"globalServiceAccountNs,omitempty"`
	AkuityIntelligence         *AkuityIntelligence      `json:"akuityIntelligence,omitempty"`
	GcConfig                   *GarbageCollectorConfig  `json:"gcConfig,omitempty"`
	PromoControllerEnabled     *bool                    `json:"promoControllerEnabled,omitempty"`
	Secrets                    SecretsManagementConfig  `json:"secrets,omitempty"`
	ArgocdUi                   *KargoArgoCDUIConfig     `json:"argocdUi,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoArgoCDUIConfig struct {
	IdpGroupsMapping *bool `json:"idpGroupsMapping,omitempty"`
}

// +kubebuilder:object:generate=true
type AkuityIntelligence struct {
	AiSupportEngineerEnabled *bool    `json:"aiSupportEngineerEnabled,omitempty"`
	Enabled                  *bool    `json:"enabled,omitempty"`
	AllowedUsernames         []string `json:"allowedUsernames,omitempty"`
	AllowedGroups            []string `json:"allowedGroups,omitempty"`
	ModelVersion             string   `json:"modelVersion,omitempty"`
}

// +kubebuilder:object:generate=true
type GarbageCollectorConfig struct {
	MaxRetainedFreight      uint32 `json:"maxRetainedFreight,omitempty"`
	MaxRetainedPromotions   uint32 `json:"maxRetainedPromotions,omitempty"`
	MinFreightDeletionAge   uint32 `json:"minFreightDeletionAge,omitempty"`
	MinPromotionDeletionAge uint32 `json:"minPromotionDeletionAge,omitempty"`
}
