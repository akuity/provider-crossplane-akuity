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

// InstanceParameters are the configurable fields of an ArgoCD Instance.
type InstanceParameters struct {
	// Name of the instance in the Akuity Platform. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ArgoCD contains the ArgoCD configuration. Required.
	// +kubebuilder:validation:Required
	ArgoCD *ArgoCD `json:"argocd"`

	// ArgoCDConfigMap sets keys in the argocd-cm ConfigMap.
	// +optional
	ArgoCDConfigMap map[string]string `json:"argocdConfigMap,omitempty"`

	// ArgoCDImageUpdaterConfigMap sets keys in the
	// argocd-image-updater-config ConfigMap.
	// +optional
	ArgoCDImageUpdaterConfigMap map[string]string `json:"argocdImageUpdaterConfigMap,omitempty"`

	// ArgoCDImageUpdaterSSHConfigMap sets keys in the
	// argocd-image-updater-ssh-config ConfigMap.
	// +optional
	ArgoCDImageUpdaterSSHConfigMap map[string]string `json:"argocdImageUpdaterSshConfigMap,omitempty"`

	// ArgoCDNotificationsConfigMap sets keys in the
	// argocd-notifications-cm ConfigMap.
	// +optional
	ArgoCDNotificationsConfigMap map[string]string `json:"argocdNotificationsConfigMap,omitempty"`

	// ArgoCDRBACConfigMap sets keys in the argocd-rbac-cm ConfigMap.
	// +optional
	ArgoCDRBACConfigMap map[string]string `json:"argocdRbacConfigMap,omitempty"`

	// ArgoCDSSHKnownHostsConfigMap sets keys in the
	// argocd-ssh-known-hosts-cm ConfigMap.
	// +optional
	ArgoCDSSHKnownHostsConfigMap map[string]string `json:"argocdSshKnownHostsConfigMap,omitempty"`

	// ArgoCDTLSCertsConfigMap sets keys in the argocd-tls-certs-cm
	// ConfigMap.
	// +optional
	ArgoCDTLSCertsConfigMap map[string]string `json:"argocdTlsCertsConfigMap,omitempty"`

	// ConfigManagementPlugins keys plugin names to their definitions.
	// +optional
	ConfigManagementPlugins map[string]ConfigManagementPlugin `json:"configManagementPlugins,omitempty"`
}

// ArgoCD mirrors the upstream ArgoCD top-level wire type.
type ArgoCD struct {
	Spec ArgoCDSpec `json:"spec,omitempty"`
}

// ArgoCDSpec is the desired configuration for an ArgoCD instance.
type ArgoCDSpec struct {
	// Description is a free-form description of the instance.
	Description string `json:"description"`
	// Version of ArgoCD to run. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
	// Shard places the instance on a specific backend shard.
	Shard string `json:"shard"`
	// InstanceSpec contains the per-instance configuration.
	// +optional
	InstanceSpec ArgoCDInstanceSpec `json:"instanceSpec,omitempty"`
}

// ArgoCDInstanceSpec curates the upstream InstanceSpec wire type. The
// v1alpha2 name is prefixed to disambiguate from the MR-level
// InstanceSpec below.
type ArgoCDInstanceSpec struct {
	// IpAllowList restricts which IPs can reach the instance.
	// +optional
	IpAllowList []*IPAllowListEntry `json:"ipAllowList,omitempty"`
	// Subdomain overrides the default Akuity subdomain.
	// +optional
	Subdomain string `json:"subdomain,omitempty"`
	// DeclarativeManagementEnabled toggles declarative management.
	// +optional
	DeclarativeManagementEnabled bool `json:"declarativeManagementEnabled,omitempty"`
	// Extensions installs ArgoCD UI extensions.
	// +optional
	Extensions []*ArgoCDExtensionInstallEntry `json:"extensions,omitempty"`
	// ClusterCustomizationDefaults defines default cluster
	// customization.
	// +optional
	ClusterCustomizationDefaults *ClusterCustomization `json:"clusterCustomizationDefaults,omitempty"`
	// ImageUpdaterEnabled toggles the ArgoCD image-updater component.
	// +optional
	ImageUpdaterEnabled bool `json:"imageUpdaterEnabled,omitempty"`
	// BackendIpAllowListEnabled toggles the backend IP allow list.
	// +optional
	BackendIpAllowListEnabled bool `json:"backendIpAllowListEnabled,omitempty"`
	// RepoServerDelegate delegates repo-server to a cluster.
	// +optional
	RepoServerDelegate *RepoServerDelegate `json:"repoServerDelegate,omitempty"`
	// AuditExtensionEnabled toggles the audit extension.
	// +optional
	AuditExtensionEnabled bool `json:"auditExtensionEnabled,omitempty"`
	// SyncHistoryExtensionEnabled toggles the sync-history extension.
	// +optional
	SyncHistoryExtensionEnabled bool `json:"syncHistoryExtensionEnabled,omitempty"`
	// CrossplaneExtension configures the Crossplane extension.
	// +optional
	CrossplaneExtension *CrossplaneExtension `json:"crossplaneExtension,omitempty"`
	// ImageUpdaterDelegate delegates image-updater to a cluster.
	// +optional
	ImageUpdaterDelegate *ImageUpdaterDelegate `json:"imageUpdaterDelegate,omitempty"`
	// AppSetDelegate delegates ApplicationSet to a cluster.
	// +optional
	AppSetDelegate *AppSetDelegate `json:"appSetDelegate,omitempty"`
	// AssistantExtensionEnabled toggles the assistant extension.
	// +optional
	AssistantExtensionEnabled bool `json:"assistantExtensionEnabled,omitempty"`
	// AppsetPolicy configures the ApplicationSet policy.
	// +optional
	AppsetPolicy *AppsetPolicy `json:"appsetPolicy,omitempty"`
	// HostAliases adds host aliases to agent pods.
	// +optional
	HostAliases []*HostAliases `json:"hostAliases,omitempty"`
	// AgentPermissionsRules are RBAC rules granted to agents.
	// +optional
	AgentPermissionsRules []*AgentPermissionsRule `json:"agentPermissionsRules,omitempty"`
	// Fqdn overrides the default instance FQDN.
	// +optional
	Fqdn string `json:"fqdn,omitempty"`
	// MultiClusterK8SDashboardEnabled toggles the multi-cluster
	// Kubernetes dashboard for the instance.
	// +optional
	MultiClusterK8SDashboardEnabled bool `json:"multiClusterK8sDashboardEnabled,omitempty"`
	// AkuityIntelligenceExtension configures the Akuity Intelligence
	// extension.
	// +optional
	AkuityIntelligenceExtension *AkuityIntelligenceExtension `json:"akuityIntelligenceExtension,omitempty"`
	// ImageUpdaterVersion pins the image-updater version.
	// +optional
	ImageUpdaterVersion string `json:"imageUpdaterVersion,omitempty"`
	// CustomDeprecatedApis lists deprecated-API shims.
	// +optional
	CustomDeprecatedApis []*CustomDeprecatedAPI `json:"customDeprecatedApis,omitempty"`
	// KubeVisionConfig configures the KubeVision extension.
	// +optional
	KubeVisionConfig *KubeVisionConfig `json:"kubeVisionConfig,omitempty"`
	// AppInAnyNamespaceConfig toggles app-in-any-namespace support.
	// +optional
	AppInAnyNamespaceConfig *AppInAnyNamespaceConfig `json:"appInAnyNamespaceConfig,omitempty"`
	// Basepath overrides the ArgoCD base path.
	// +optional
	Basepath string `json:"basepath,omitempty"`
	// AppsetProgressiveSyncsEnabled toggles AppSet progressive syncs.
	// +optional
	AppsetProgressiveSyncsEnabled bool `json:"appsetProgressiveSyncsEnabled,omitempty"`
	// Secrets configures cross-cluster secret mappings.
	// +optional
	Secrets *SecretsManagementConfig `json:"secrets,omitempty"`
	// AppsetPlugins registers AppSet plugins.
	// +optional
	AppsetPlugins []*AppsetPlugins `json:"appsetPlugins,omitempty"`
	// ApplicationSetExtension configures the ApplicationSet extension.
	// +optional
	ApplicationSetExtension *ApplicationSetExtension `json:"applicationSetExtension,omitempty"`
	// AppReconciliationsRateLimiting tunes reconcile rate limiting.
	// +optional
	AppReconciliationsRateLimiting *AppReconciliationsRateLimiting `json:"appReconciliationsRateLimiting,omitempty"`
	// MetricsIngressUsername is the basic-auth username for the
	// metrics ingress.
	// +optional
	MetricsIngressUsername *string `json:"metricsIngressUsername,omitempty"`
	// MetricsIngressPasswordHash is the htpasswd-hashed password for
	// the metrics ingress.
	// +optional
	MetricsIngressPasswordHash *string `json:"metricsIngressPasswordHash,omitempty"`
	// PrivilegedNotificationCluster selects a cluster that may run
	// privileged notifications.
	// +optional
	PrivilegedNotificationCluster *string `json:"privilegedNotificationCluster,omitempty"`
	// ClusterAddonsExtension configures the cluster-addons extension.
	// +optional
	ClusterAddonsExtension *ClusterAddonsExtension `json:"clusterAddonsExtension,omitempty"`
	// ManifestGeneration configures manifest-generation tool versions.
	// +optional
	ManifestGeneration *ManifestGeneration `json:"manifestGeneration,omitempty"`
}

// ArgoCDExtensionInstallEntry installs an ArgoCD UI extension.
type ArgoCDExtensionInstallEntry struct {
	// Id of the extension.
	// +optional
	Id string `json:"id,omitempty"`
	// Version of the extension to install.
	// +optional
	Version string `json:"version,omitempty"`
}

// ClusterCustomization defines default cluster customization.
type ClusterCustomization struct {
	// AutoUpgradeDisabled disables automatic agent upgrades.
	// +optional
	AutoUpgradeDisabled bool `json:"autoUpgradeDisabled,omitempty"`
	// Kustomization YAML applied to agent manifests.
	// +optional
	Kustomization string `json:"kustomization,omitempty"`
	// AppReplication toggles app replication.
	// +optional
	AppReplication bool `json:"appReplication,omitempty"`
	// RedisTunneling toggles Redis tunneling.
	// +optional
	RedisTunneling bool `json:"redisTunneling,omitempty"`
	// ServerSideDiffEnabled toggles server-side diff.
	// +optional
	ServerSideDiffEnabled bool `json:"serverSideDiffEnabled,omitempty"`
}

// AppsetPolicy configures ApplicationSet behavior.
type AppsetPolicy struct {
	// Policy name applied to ApplicationSets.
	// +optional
	Policy string `json:"policy,omitempty"`
	// OverridePolicy allows overrides of the policy.
	// +optional
	OverridePolicy bool `json:"overridePolicy,omitempty"`
}

// AgentPermissionsRule is a Kubernetes-RBAC-shaped rule granted to
// agents.
type AgentPermissionsRule struct {
	// ApiGroups granted.
	// +optional
	ApiGroups []string `json:"apiGroups,omitempty"`
	// Resources granted.
	// +optional
	Resources []string `json:"resources,omitempty"`
	// Verbs granted.
	// +optional
	Verbs []string `json:"verbs,omitempty"`
}

// CrossplaneExtensionResource names a Crossplane resource group
// consumed by the Crossplane extension.
type CrossplaneExtensionResource struct {
	// Group of the Crossplane resource.
	// +optional
	Group string `json:"group,omitempty"`
}

// CrossplaneExtension configures the Crossplane extension.
type CrossplaneExtension struct {
	// Resources lists Crossplane resource groups to expose.
	// +optional
	Resources []*CrossplaneExtensionResource `json:"resources,omitempty"`
}

// AkuityIntelligenceExtension configures the Akuity Intelligence
// extension.
type AkuityIntelligenceExtension struct {
	// Enabled toggles the extension.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// AllowedUsernames grants access to named users.
	// +optional
	AllowedUsernames []string `json:"allowedUsernames,omitempty"`
	// AllowedGroups grants access to named groups.
	// +optional
	AllowedGroups []string `json:"allowedGroups,omitempty"`
	// AiSupportEngineerEnabled toggles the AI support engineer persona.
	// +optional
	AiSupportEngineerEnabled bool `json:"aiSupportEngineerEnabled,omitempty"`
	// ModelVersion pins the AI model version.
	// +optional
	ModelVersion string `json:"modelVersion,omitempty"`
}

// ClusterAddonsExtension configures the cluster-addons extension.
type ClusterAddonsExtension struct {
	// Enabled toggles the extension.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// AllowedUsernames grants access to named users.
	// +optional
	AllowedUsernames []string `json:"allowedUsernames,omitempty"`
	// AllowedGroups grants access to named groups.
	// +optional
	AllowedGroups []string `json:"allowedGroups,omitempty"`
}

// TargetSelector selects a subset of ArgoCD applications, k8s
// namespaces, or clusters.
type TargetSelector struct {
	// ArgocdApplications selects apps by name.
	// +optional
	ArgocdApplications []string `json:"argocdApplications,omitempty"`
	// K8SNamespaces selects Kubernetes namespaces.
	// +optional
	K8SNamespaces []string `json:"k8sNamespaces,omitempty"`
	// Clusters selects clusters by name.
	// +optional
	Clusters []string `json:"clusters,omitempty"`
	// DegradedFor restricts matches to resources degraded for at least
	// this duration.
	// +optional
	DegradedFor *string `json:"degradedFor,omitempty"`
}

// Runbook defines a KubeVision runbook.
type Runbook struct {
	// Name of the runbook.
	// +optional
	Name string `json:"name,omitempty"`
	// Content of the runbook (markdown).
	// +optional
	Content string `json:"content,omitempty"`
	// AppliedTo scopes the runbook.
	// +optional
	AppliedTo *TargetSelector `json:"appliedTo,omitempty"`
	// SlackChannelNames route the runbook to Slack channels.
	// +optional
	SlackChannelNames []string `json:"slackChannelNames,omitempty"`
}

// IncidentWebhookConfig configures an incident webhook.
type IncidentWebhookConfig struct {
	// Name of the webhook.
	// +optional
	Name string `json:"name,omitempty"`
	// DescriptionPath in the webhook payload.
	// +optional
	DescriptionPath string `json:"descriptionPath,omitempty"`
	// ClusterPath in the webhook payload.
	// +optional
	ClusterPath string `json:"clusterPath,omitempty"`
	// K8SNamespacePath in the webhook payload.
	// +optional
	K8SNamespacePath string `json:"k8sNamespacePath,omitempty"`
	// ArgocdApplicationNamePath in the webhook payload.
	// +optional
	ArgocdApplicationNamePath string `json:"argocdApplicationNamePath,omitempty"`
	// ArgocdApplicationNamespacePath in the webhook payload.
	// +optional
	ArgocdApplicationNamespacePath string `json:"argocdApplicationNamespacePath,omitempty"`
	// TitlePath in the webhook payload.
	// +optional
	TitlePath string `json:"titlePath,omitempty"`
}

// IncidentsGroupingConfig groups incidents.
type IncidentsGroupingConfig struct {
	// K8SNamespaces groups incidents by namespace.
	// +optional
	K8SNamespaces []string `json:"k8sNamespaces,omitempty"`
	// ArgocdApplicationNames groups incidents by ArgoCD app name.
	// +optional
	ArgocdApplicationNames []string `json:"argocdApplicationNames,omitempty"`
}

// IncidentInvestigationApprovalScope scopes auto-closure approvals.
type IncidentInvestigationApprovalScope struct {
	// ArgocdApplications selects apps.
	// +optional
	ArgocdApplications []string `json:"argocdApplications,omitempty"`
	// K8SNamespaces selects namespaces.
	// +optional
	K8SNamespaces []string `json:"k8sNamespaces,omitempty"`
	// Clusters selects clusters.
	// +optional
	Clusters []string `json:"clusters,omitempty"`
	// ConsecutiveAutoClosures threshold before approval is required.
	// +optional
	ConsecutiveAutoClosures int32 `json:"consecutiveAutoClosures,omitempty"`
}

// IncidentInvestigationApprovalConfig configures investigation
// approval scopes.
type IncidentInvestigationApprovalConfig struct {
	// Scopes define approval thresholds.
	// +optional
	Scopes []*IncidentInvestigationApprovalScope `json:"scopes,omitempty"`
}

// IncidentsConfig configures incident detection.
type IncidentsConfig struct {
	// Triggers are target selectors that fire incidents.
	// +optional
	Triggers []*TargetSelector `json:"triggers,omitempty"`
	// Webhooks route incidents to external systems.
	// +optional
	Webhooks []*IncidentWebhookConfig `json:"webhooks,omitempty"`
	// Grouping rules group related incidents.
	// +optional
	Grouping *IncidentsGroupingConfig `json:"grouping,omitempty"`
	// InvestigationApproval configures approval thresholds.
	// +optional
	InvestigationApproval *IncidentInvestigationApprovalConfig `json:"investigationApproval,omitempty"`
}

// RunbookRepo references a runbook repository.
type RunbookRepo struct {
	// RepoUrl of the runbook repository.
	// +optional
	RepoUrl string `json:"repoUrl,omitempty"`
	// Revision checked out.
	// +optional
	Revision *string `json:"revision,omitempty"`
	// Path within the repository.
	// +optional
	Path *string `json:"path,omitempty"`
	// AppliedFor maps runbook names to scoping selectors.
	// +optional
	AppliedFor map[string]*TargetSelector `json:"appliedFor,omitempty"`
}

// AIConfig configures KubeVision's AI features.
type AIConfig struct {
	// Runbooks are KubeVision runbooks.
	// +optional
	Runbooks []*Runbook `json:"runbooks,omitempty"`
	// Incidents configures incident detection.
	// +optional
	Incidents *IncidentsConfig `json:"incidents,omitempty"`
	// ArgocdSlackService selects the ArgoCD slack notification service.
	// +optional
	ArgocdSlackService *string `json:"argocdSlackService,omitempty"`
	// ArgocdSlackChannels routes ArgoCD notifications.
	// +optional
	ArgocdSlackChannels []string `json:"argocdSlackChannels,omitempty"`
	// RunbookRepos configures runbook repositories.
	// +optional
	RunbookRepos []*RunbookRepo `json:"runbookRepos,omitempty"`
}

// AdditionalAttributeRule attaches extra attributes to a resource
// kind.
type AdditionalAttributeRule struct {
	// Group of the resource.
	// +optional
	Group string `json:"group,omitempty"`
	// Kind of the resource.
	// +optional
	Kind string `json:"kind,omitempty"`
	// Annotations copied onto matching resources.
	// +optional
	Annotations []string `json:"annotations,omitempty"`
	// Labels copied onto matching resources.
	// +optional
	Labels []string `json:"labels,omitempty"`
	// Namespace scoping for the rule.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// KubeVisionConfig configures the KubeVision extension.
type KubeVisionConfig struct {
	// CveScanConfig configures CVE scanning.
	// +optional
	CveScanConfig *CveScanConfig `json:"cveScanConfig,omitempty"`
	// AiConfig configures AI features.
	// +optional
	AiConfig *AIConfig `json:"aiConfig,omitempty"`
	// AdditionalAttributes configures extra attribute copying.
	// +optional
	AdditionalAttributes []*AdditionalAttributeRule `json:"additionalAttributes,omitempty"`
}

// AppInAnyNamespaceConfig toggles app-in-any-namespace support.
type AppInAnyNamespaceConfig struct {
	// Enabled toggles the feature.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// CustomDeprecatedAPI shim maps a deprecated API to a replacement.
type CustomDeprecatedAPI struct {
	// ApiVersion of the deprecated API.
	// +optional
	ApiVersion string `json:"apiVersion,omitempty"`
	// NewApiVersion replaces the deprecated ApiVersion.
	// +optional
	NewApiVersion string `json:"newApiVersion,omitempty"`
	// DeprecatedInKubernetesVersion records when it became deprecated.
	// +optional
	DeprecatedInKubernetesVersion string `json:"deprecatedInKubernetesVersion,omitempty"`
	// UnavailableInKubernetesVersion records when it becomes removed.
	// +optional
	UnavailableInKubernetesVersion string `json:"unavailableInKubernetesVersion,omitempty"`
}

// CveScanConfig configures CVE scanning.
type CveScanConfig struct {
	// ScanEnabled toggles scanning.
	// +optional
	ScanEnabled bool `json:"scanEnabled,omitempty"`
	// RescanInterval configures the rescan period (Go duration).
	// +optional
	RescanInterval string `json:"rescanInterval,omitempty"`
}

// ObjectSelector is a Kubernetes-style label selector.
type ObjectSelector struct {
	// MatchLabels selects by label equality.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// MatchExpressions selects by compound label expression.
	// +optional
	MatchExpressions []*LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// LabelSelectorRequirement is a single label-selector expression.
type LabelSelectorRequirement struct {
	// Key is the label key.
	// +optional
	Key *string `json:"key,omitempty"`
	// Operator applied to values.
	// +optional
	Operator *string `json:"operator,omitempty"`
	// Values compared against the label.
	// +optional
	Values []string `json:"values,omitempty"`
}

// ClusterSecretMapping maps clusters to secrets.
type ClusterSecretMapping struct {
	// Clusters selects source/destination clusters.
	// +optional
	Clusters *ObjectSelector `json:"clusters,omitempty"`
	// Secrets selects secret resources.
	// +optional
	Secrets *ObjectSelector `json:"secrets,omitempty"`
}

// SecretsManagementConfig configures cross-cluster secret management.
type SecretsManagementConfig struct {
	// Sources identify source mappings.
	// +optional
	Sources []*ClusterSecretMapping `json:"sources,omitempty"`
	// Destinations identify destination mappings.
	// +optional
	Destinations []*ClusterSecretMapping `json:"destinations,omitempty"`
}

// ApplicationSetExtension configures the ApplicationSet extension.
type ApplicationSetExtension struct {
	// Enabled toggles the extension.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// BucketRateLimiting bounds global reconcile throughput.
type BucketRateLimiting struct {
	// Enabled toggles bucket rate limiting.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// BucketSize is the token-bucket capacity.
	// +optional
	BucketSize uint32 `json:"bucketSize,omitempty"`
	// BucketQps is the refill rate.
	// +optional
	BucketQps uint32 `json:"bucketQps,omitempty"`
}

// ItemRateLimiting configures per-item (app) backoff.
type ItemRateLimiting struct {
	// Enabled toggles item rate limiting.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// FailureCooldown seconds after a terminal failure.
	// +optional
	FailureCooldown uint32 `json:"failureCooldown,omitempty"`
	// BaseDelay seconds for the first retry.
	// +optional
	BaseDelay uint32 `json:"baseDelay,omitempty"`
	// MaxDelay seconds between retries.
	// +optional
	MaxDelay uint32 `json:"maxDelay,omitempty"`
	// BackoffFactor multiplies each subsequent delay.
	// +optional
	BackoffFactor float32 `json:"backoffFactor,omitempty"`
}

// AppReconciliationsRateLimiting tunes reconcile rate limiting.
type AppReconciliationsRateLimiting struct {
	// BucketRateLimiting bounds global throughput.
	// +optional
	BucketRateLimiting *BucketRateLimiting `json:"bucketRateLimiting,omitempty"`
	// ItemRateLimiting configures per-item backoff.
	// +optional
	ItemRateLimiting *ItemRateLimiting `json:"itemRateLimiting,omitempty"`
}

// ConfigManagementToolVersions pins config-management tool versions.
type ConfigManagementToolVersions struct {
	// DefaultVersion selected by default.
	// +optional
	DefaultVersion string `json:"defaultVersion,omitempty"`
	// AdditionalVersions available alongside the default.
	// +optional
	AdditionalVersions []string `json:"additionalVersions,omitempty"`
}

// ManifestGeneration configures manifest-generation tool versions.
type ManifestGeneration struct {
	// Kustomize versions available.
	// +optional
	Kustomize *ConfigManagementToolVersions `json:"kustomize,omitempty"`
}

// AppsetPlugins registers an ApplicationSet plugin.
type AppsetPlugins struct {
	// Name of the plugin.
	// +optional
	Name string `json:"name,omitempty"`
	// Token credential used for the plugin API.
	// +optional
	Token string `json:"token,omitempty"`
	// BaseUrl is the plugin API endpoint.
	// +optional
	BaseUrl string `json:"baseUrl,omitempty"`
	// RequestTimeout in seconds.
	// +optional
	RequestTimeout int32 `json:"requestTimeout,omitempty"`
}

// ManagedCluster names a cluster that hosts a delegated component.
type ManagedCluster struct {
	// ClusterName is the name of the managed cluster.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
}

// RepoServerDelegate delegates ArgoCD repo-server to a cluster.
type RepoServerDelegate struct {
	// ControlPlane runs repo-server on the Akuity control plane.
	// +optional
	ControlPlane bool `json:"controlPlane,omitempty"`
	// ManagedCluster runs repo-server on a managed cluster.
	// +optional
	ManagedCluster *ManagedCluster `json:"managedCluster,omitempty"`
}

// ImageUpdaterDelegate delegates image-updater to a cluster.
type ImageUpdaterDelegate struct {
	// ControlPlane runs image-updater on the Akuity control plane.
	// +optional
	ControlPlane bool `json:"controlPlane,omitempty"`
	// ManagedCluster runs image-updater on a managed cluster.
	// +optional
	ManagedCluster *ManagedCluster `json:"managedCluster,omitempty"`
}

// AppSetDelegate delegates ApplicationSet to a managed cluster.
type AppSetDelegate struct {
	// ManagedCluster hosts ApplicationSet.
	// +optional
	ManagedCluster *ManagedCluster `json:"managedCluster,omitempty"`
}

// IPAllowListEntry grants access for a single IP or CIDR.
type IPAllowListEntry struct {
	// Ip is the IP address or CIDR.
	// +optional
	Ip string `json:"ip,omitempty"`
	// Description is free-form context.
	// +optional
	Description string `json:"description,omitempty"`
}

// HostAliases adds /etc/hosts-style host aliases to agent pods.
type HostAliases struct {
	// Ip is the host IP.
	// +optional
	Ip string `json:"ip,omitempty"`
	// Hostnames resolve to Ip.
	// +optional
	Hostnames []string `json:"hostnames,omitempty"`
}

// ConfigManagementPlugin registers a v2 config-management plugin.
type ConfigManagementPlugin struct {
	// Enabled toggles the plugin.
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`
	// Image containing the plugin binary.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`
	// Spec is the plugin definition.
	// +optional
	Spec PluginSpec `json:"spec,omitempty"`
}

// PluginSpec describes a v2 plugin.
type PluginSpec struct {
	// Version of the plugin API.
	// +optional
	Version string `json:"version,omitempty"`
	// Init command executed before generation.
	// +optional
	Init *Command `json:"init,omitempty"`
	// Generate command executed to render manifests.
	// +optional
	Generate *Command `json:"generate,omitempty"`
	// Discover configures plugin activation.
	// +optional
	Discover *Discover `json:"discover,omitempty"`
	// Parameters announces plugin parameters to the UI.
	// +optional
	Parameters *Parameters `json:"parameters,omitempty"`
	// PreserveFileMode retains source-file modes in rendered output.
	// +optional
	PreserveFileMode bool `json:"preserveFileMode,omitempty"`
}

// Command describes a shell invocation.
type Command struct {
	// Command argv[0..].
	// +optional
	Command []string `json:"command,omitempty"`
	// Args passed to Command.
	// +optional
	Args []string `json:"args,omitempty"`
}

// Discover configures plugin activation on a given app.
type Discover struct {
	// Find configures discovery via exec or glob.
	// +optional
	Find *Find `json:"find,omitempty"`
	// FileName triggers activation when present.
	// +optional
	FileName string `json:"fileName,omitempty"`
}

// Find describes plugin discovery.
type Find struct {
	// Command argv to run for discovery.
	// +optional
	Command []string `json:"command,omitempty"`
	// Args passed to Command.
	// +optional
	Args []string `json:"args,omitempty"`
	// Glob pattern used for discovery.
	// +optional
	Glob string `json:"glob,omitempty"`
}

// Parameters lists static and dynamic plugin parameters.
type Parameters struct {
	// Static parameter announcements.
	// +optional
	Static []*ParameterAnnouncement `json:"static,omitempty"`
	// Dynamic parameter generator.
	// +optional
	Dynamic *Dynamic `json:"dynamic,omitempty"`
}

// Dynamic parameter generator spec.
type Dynamic struct {
	// Command argv.
	// +optional
	Command []string `json:"command,omitempty"`
	// Args passed to Command.
	// +optional
	Args []string `json:"args,omitempty"`
}

// ParameterAnnouncement declares a single plugin parameter.
type ParameterAnnouncement struct {
	// Name of the parameter.
	// +optional
	Name string `json:"name,omitempty"`
	// Title shown in the UI.
	// +optional
	Title string `json:"title,omitempty"`
	// Tooltip shown on hover.
	// +optional
	Tooltip string `json:"tooltip,omitempty"`
	// Required marks the parameter as required.
	// +optional
	Required bool `json:"required,omitempty"`
	// ItemType hints the primitive type of each value.
	// +optional
	ItemType string `json:"itemType,omitempty"`
	// CollectionType hints the collection shape.
	// +optional
	CollectionType string `json:"collectionType,omitempty"`
	// String default value.
	// +optional
	String string `json:"string,omitempty"`
	// Array default value.
	// +optional
	Array []string `json:"array,omitempty"`
	// Map default value.
	// +optional
	Map map[string]string `json:"map,omitempty"`
}

// InstanceObservation are the observable fields of an Instance.
type InstanceObservation struct {
	// ID assigned by the Akuity Platform.
	ID string `json:"id,omitempty"`
	// Name of the instance.
	Name string `json:"name,omitempty"`
	// Hostname is the public hostname.
	Hostname string `json:"hostname,omitempty"`
	// ClusterCount is the number of managed clusters attached.
	ClusterCount uint32 `json:"clusterCount,omitempty"`
	// HealthStatus is the instance health.
	HealthStatus ResourceStatusCode `json:"healthStatus,omitempty"`
	// ReconciliationStatus is the instance reconciliation status.
	ReconciliationStatus ResourceStatusCode `json:"reconciliationStatus,omitempty"`
	// OwnerOrganizationName is the Akuity organization owning the
	// instance.
	OwnerOrganizationName string `json:"ownerOrganizationName,omitempty"`
	// ArgoCD is the observed ArgoCD configuration.
	ArgoCD ArgoCD `json:"argocd,omitempty"`
	// ArgoCDConfigMap is the observed argocd-cm ConfigMap.
	ArgoCDConfigMap map[string]string `json:"argocdConfigMap,omitempty"`
	// ArgoCDImageUpdaterConfigMap is the observed
	// argocd-image-updater-config ConfigMap.
	ArgoCDImageUpdaterConfigMap map[string]string `json:"argocdImageUpdaterConfigMap,omitempty"`
	// ArgoCDImageUpdaterSSHConfigMap is the observed
	// argocd-image-updater-ssh-config ConfigMap.
	ArgoCDImageUpdaterSSHConfigMap map[string]string `json:"argocdImageUpdaterSshConfigMap,omitempty"`
	// ArgoCDNotificationsConfigMap is the observed
	// argocd-notifications-cm ConfigMap.
	ArgoCDNotificationsConfigMap map[string]string `json:"argocdNotificationsConfigMap,omitempty"`
	// ArgoCDRBACConfigMap is the observed argocd-rbac-cm ConfigMap.
	ArgoCDRBACConfigMap map[string]string `json:"argocdRbacConfigMap,omitempty"`
	// ArgoCDSSHKnownHostsConfigMap is the observed
	// argocd-ssh-known-hosts-cm ConfigMap.
	ArgoCDSSHKnownHostsConfigMap map[string]string `json:"argocdSshKnownHostsConfigMap,omitempty"`
	// ArgoCDTLSCertsConfigMap is the observed argocd-tls-certs-cm
	// ConfigMap.
	ArgoCDTLSCertsConfigMap map[string]string `json:"argocdTlsCertsConfigMap,omitempty"`
	// ConfigManagementPlugins is the observed plugin set.
	ConfigManagementPlugins map[string]ConfigManagementPlugin `json:"configManagementPlugins,omitempty"`
}

// An InstanceSpec defines the desired state of an Instance.
type InstanceSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              InstanceParameters `json:"forProvider"`
}

// An InstanceStatus represents the observed state of an Instance.
type InstanceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          InstanceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// An Instance is an Akuity ArgoCD managed instance.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,akuity}
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSpec   `json:"spec"`
	Status InstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceList contains a list of Instance.
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
