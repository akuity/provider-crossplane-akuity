// This is an auto-generated file. DO NOT EDIT
/*
Copyright 2026 Akuity, Inc.
*/

package v1alpha1

import xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

// +kubebuilder:object:generate=true
type Kargo struct {
	Spec KargoSpec `json:"spec,omitempty"`
}

// KargoOidcConfig is emitted from the template (not from codegenStructs)
// because it carries a provider-specific overlay the Akuity Crossplane
// provider needs but the upstream API source type does not define:
// DexConfigSecretRef, a namespaced Secret reference resolved at
// reconcile time into the wire-shape DexConfigSecret map. Plaintext
// dex credentials never live on the managed resource spec.
//
// +kubebuilder:object:generate=true
type KargoOidcConfig struct {
	Enabled         *bool            `json:"enabled,omitempty"`
	DexEnabled      *bool            `json:"dexEnabled,omitempty"`
	DexConfig       string           `json:"dexConfig,omitempty"`
	DexConfigSecret map[string]Value `json:"dexConfigSecret,omitempty"`
	// DexConfigSecretRef references a namespaced Secret whose data is
	// resolved at reconcile time by the Crossplane provider and
	// forwarded to the Akuity gateway as the wire-shape
	// DexConfigSecret map. Mutually exclusive with the inline
	// DexConfigSecret above. Removing this ref stops applying the
	// platform-side Secret, but does not delete it from the Akuity
	// platform.
	DexConfigSecretRef    *xpv1.SecretReference      `json:"dexConfigSecretRef,omitempty"`
	IssuerURL             string                     `json:"issuerUrl,omitempty"`
	ClientID              string                     `json:"clientId,omitempty"`
	CliClientID           string                     `json:"cliClientId,omitempty"`
	AdminAccount          KargoPredefinedAccountData `json:"adminAccount,omitempty"`
	ViewerAccount         KargoPredefinedAccountData `json:"viewerAccount,omitempty"`
	AdditionalScopes      []string                   `json:"additionalScopes,omitempty"`
	UserAccount           KargoPredefinedAccountData `json:"userAccount,omitempty"`
	ProjectCreatorAccount KargoPredefinedAccountData `json:"projectCreatorAccount,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoSpec struct {
	Description       string            `json:"description"`
	Version           string            `json:"version"`
	Shard             string            `json:"shard,omitempty"`
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
	AutoUpgradeDisabled *bool        `json:"autoUpgradeDisabled,omitempty"`
	Kustomization       string       `json:"kustomization,omitempty"`
	Connectivity        Connectivity `json:"connectivity,omitempty"`
	CustomCaBundle      string       `json:"customCaBundle,omitempty"`
}

// +kubebuilder:object:generate=true
type KargoInstanceSpec struct {
	BackendIpAllowListEnabled    *bool                    `json:"backendIpAllowListEnabled,omitempty"`
	IpAllowList                  []*KargoIPAllowListEntry `json:"ipAllowList,omitempty"`
	AgentCustomizationDefaults   *KargoAgentCustomization `json:"agentCustomizationDefaults,omitempty"`
	DefaultShardAgent            string                   `json:"defaultShardAgent,omitempty"`
	GlobalCredentialsNs          []string                 `json:"globalCredentialsNs,omitempty"`
	GlobalServiceAccountNs       []string                 `json:"globalServiceAccountNs,omitempty"`
	AkuityIntelligence           *AkuityIntelligence      `json:"akuityIntelligence,omitempty"`
	GcConfig                     *GarbageCollectorConfig  `json:"gcConfig,omitempty"`
	PromoControllerEnabled       *bool                    `json:"promoControllerEnabled,omitempty"`
	Secrets                      SecretsManagementConfig  `json:"secrets,omitempty"`
	ArgocdUi                     *KargoArgoCDUIConfig     `json:"argocdUi,omitempty"`
	TerminationProtectionEnabled *bool                    `json:"terminationProtectionEnabled,omitempty"`
	TerminationProtectionNotes   *string                  `json:"terminationProtectionNotes,omitempty"`
	Connectivity                 Connectivity             `json:"connectivity,omitempty"`
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
