package fixtures

import (
	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

var (
	InstanceID   = "test-instance-id"
	InstanceName = "test-instance"

	AkuityHostAliasesList = []*akuitytypes.HostAliases{
		{
			Ip:        "192.168.0.1",
			Hostnames: []string{"example.com", "www.example.com"},
		},
		{
			Ip:        "192.168.0.2",
			Hostnames: []string{"test.com", "www.test.com"},
		},
	}

	ArgocdHostAliasesList = []*argocdv1.HostAliases{
		{
			Ip:        "192.168.0.1",
			Hostnames: []string{"example.com", "www.example.com"},
		},
		{
			Ip:        "192.168.0.2",
			Hostnames: []string{"test.com", "www.test.com"},
		},
	}

	CrossplaneHostAliasesList = []*crossplanetypes.HostAliases{
		{
			Ip:        "192.168.0.1",
			Hostnames: []string{"example.com", "www.example.com"},
		},
		{
			Ip:        "192.168.0.2",
			Hostnames: []string{"test.com", "www.test.com"},
		},
	}

	AkuityAppsetPolicy = &akuitytypes.AppsetPolicy{
		Policy:         "policy",
		OverridePolicy: new(true),
	}

	ArgocdAppsetPolicy = &argocdv1.AppsetPolicy{
		Policy:         "policy",
		OverridePolicy: true,
	}

	CrossplaneAppsetPolicy = &crossplanetypes.AppsetPolicy{
		Policy:         "policy",
		OverridePolicy: new(true),
	}

	AkuityAppSetDelegate = &akuitytypes.AppSetDelegate{
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	ArgocdAppSetDelegate = &argocdv1.AppSetDelegate{
		ManagedCluster: &argocdv1.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	CrossplaneAppSetDelegate = &crossplanetypes.AppSetDelegate{
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	AkuityImageUpdaterDelegate = &akuitytypes.ImageUpdaterDelegate{
		ControlPlane: new(true),
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	ArgocdImageUpdaterDelegate = &argocdv1.ImageUpdaterDelegate{
		ControlPlane: true,
		ManagedCluster: &argocdv1.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	CrossplaneImageUpdateDelegate = &crossplanetypes.ImageUpdaterDelegate{
		ControlPlane: new(true),
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	AkuityRepoServerDelegate = &akuitytypes.RepoServerDelegate{
		ControlPlane: new(true),
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	ArgocdRepoServerDelegate = &argocdv1.RepoServerDelegate{
		ControlPlane: true,
		ManagedCluster: &argocdv1.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	CrossplaneRepoServerDelegate = &crossplanetypes.RepoServerDelegate{
		ControlPlane: new(true),
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	AkuityInstallEntryList = []*akuitytypes.ArgoCDExtensionInstallEntry{
		{
			Id:      "test-id-1",
			Version: "test-version-1",
		},
		{
			Id:      "test-id-2",
			Version: "test-version-2",
		},
	}

	ArgocdInstallEntryList = []*argocdv1.ArgoCDExtensionInstallEntry{
		{
			Id:      "test-id-1",
			Version: "test-version-1",
		},
		{
			Id:      "test-id-2",
			Version: "test-version-2",
		},
	}

	CrossplaneInstallEntryList = []*crossplanetypes.ArgoCDExtensionInstallEntry{
		{
			Id:      "test-id-1",
			Version: "test-version-1",
		},
		{
			Id:      "test-id-2",
			Version: "test-version-2",
		},
	}

	ArgocdIpAllowList = []*argocdv1.IPAllowListEntry{
		{
			Ip:          "192.168.0.1",
			Description: "Allowlist entry 1",
		},
		{
			Ip:          "192.168.0.2",
			Description: "Allowlist entry 2",
		},
	}

	AkuityIpAllowList = []*akuitytypes.IPAllowListEntry{
		{
			Ip:          "192.168.0.1",
			Description: "Allowlist entry 1",
		},
		{
			Ip:          "192.168.0.2",
			Description: "Allowlist entry 2",
		},
	}

	CrossplaneIpAllowList = []*crossplanetypes.IPAllowListEntry{
		{
			Ip:          "192.168.0.1",
			Description: "Allowlist entry 1",
		},
		{
			Ip:          "192.168.0.2",
			Description: "Allowlist entry 2",
		},
	}

	AkuityClusterCustomization = &akuitytypes.ClusterCustomization{
		AutoUpgradeDisabled: new(true),
		Kustomization:       Kustomization,
		AppReplication:      new(true),
		RedisTunneling:      new(true),
	}

	ArgocdClusterCustomization = &argocdv1.ClusterCustomization{
		AutoUpgradeDisabled: true,
		AppReplication:      true,
		RedisTunneling:      true,
		Kustomization:       KustomizationPB,
	}

	CrossplaneClusterCustomization = &crossplanetypes.ClusterCustomization{
		AutoUpgradeDisabled:   new(true),
		Kustomization:         KustomizationYAML,
		AppReplication:        new(true),
		RedisTunneling:        new(true),
		ServerSideDiffEnabled: new(false),
	}

	AkuityIntelligenceExtension = &akuitytypes.AkuityIntelligenceExtension{
		Enabled:                  new(true),
		AllowedGroups:            []string{"admin"},
		AllowedUsernames:         []string{"admin"},
		AiSupportEngineerEnabled: new(true),
		ModelVersion:             "gpt-4.1",
	}

	ArgocdIntelligenceExtension = &argocdv1.AkuityIntelligenceExtension{
		Enabled:                  true,
		AllowedGroups:            []string{"admin"},
		AllowedUsernames:         []string{"admin"},
		AiSupportEngineerEnabled: true,
		ModelVersion:             "gpt-4.1",
	}

	CrossplaneIntelligenceExtension = &crossplanetypes.AkuityIntelligenceExtension{
		Enabled:                  new(true),
		AllowedGroups:            []string{"admin"},
		AllowedUsernames:         []string{"admin"},
		AiSupportEngineerEnabled: new(true),
		ModelVersion:             "gpt-4.1",
	}

	AkuityCustomDeprecatedApis = []*akuitytypes.CustomDeprecatedAPI{
		{
			ApiVersion:                     "v1",
			NewApiVersion:                  "v2",
			DeprecatedInKubernetesVersion:  "1.18",
			UnavailableInKubernetesVersion: "1.19",
		},
	}

	ArgocdCustomDeprecatedApis = []*argocdv1.CustomDeprecatedAPI{
		{
			ApiVersion:                     "v1",
			NewApiVersion:                  "v2",
			DeprecatedInKubernetesVersion:  "1.18",
			UnavailableInKubernetesVersion: "1.19",
		},
	}

	CrossplaneCustomDeprecatedApis = []*crossplanetypes.CustomDeprecatedAPI{
		{
			ApiVersion:                     "v1",
			NewApiVersion:                  "v2",
			DeprecatedInKubernetesVersion:  "1.18",
			UnavailableInKubernetesVersion: "1.19",
		},
	}

	AkuityKubeVisionConfig = &akuitytypes.KubeVisionConfig{
		CveScanConfig: &akuitytypes.CveScanConfig{
			ScanEnabled:    new(true),
			RescanInterval: "",
		},
	}

	ArgocdKubeVisionConfig = &argocdv1.KubeVisionConfig{
		CveScanConfig: &argocdv1.CveScanConfig{
			ScanEnabled:    true,
			RescanInterval: "",
		},
	}

	CrossplaneKubeVisionConfig = &crossplanetypes.KubeVisionConfig{
		CveScanConfig: &crossplanetypes.CveScanConfig{
			ScanEnabled:    new(true),
			RescanInterval: "",
		},
	}

	AkuityAppInAnyNamespaceConfig = &akuitytypes.AppInAnyNamespaceConfig{
		Enabled: new(true),
	}

	ArgocdAppInAnyNamespaceConfig = &argocdv1.AppInAnyNamespaceConfig{
		Enabled: true,
	}

	CrossplaneAppInAnyNamespaceConfig = &crossplanetypes.AppInAnyNamespaceConfig{
		Enabled: new(true),
	}

	AkuityAppsetPlugins = []*akuitytypes.AppsetPlugins{
		{
			Name:           "plugin-1",
			Token:          "token-1",
			BaseUrl:        "https://plugins.example.com",
			RequestTimeout: 30,
		},
		{
			Name:           "plugin-2",
			Token:          "token-2",
			BaseUrl:        "https://plugins.example.com",
			RequestTimeout: 45,
		},
	}

	ArgocdAppsetPlugins = []*argocdv1.AppsetPlugins{
		{
			Name:           "plugin-1",
			Token:          "token-1",
			BaseUrl:        "https://plugins.example.com",
			RequestTimeout: 30,
		},
		{
			Name:           "plugin-2",
			Token:          "token-2",
			BaseUrl:        "https://plugins.example.com",
			RequestTimeout: 45,
		},
	}

	CrossplaneAppsetPlugins = []*crossplanetypes.AppsetPlugins{
		{
			Name:           "plugin-1",
			Token:          "token-1",
			BaseUrl:        "https://plugins.example.com",
			RequestTimeout: 30,
		},
		{
			Name:           "plugin-2",
			Token:          "token-2",
			BaseUrl:        "https://plugins.example.com",
			RequestTimeout: 45,
		},
	}

	AkuityApplicationSetExtension = &akuitytypes.ApplicationSetExtension{
		Enabled: new(true),
	}

	ArgocdApplicationSetExtension = &argocdv1.ApplicationSetExtension{
		Enabled: true,
	}

	CrossplaneApplicationSetExtension = &crossplanetypes.ApplicationSetExtension{
		Enabled: new(true),
	}

	AkuityAppReconciliationsRateLimiting = &akuitytypes.AppReconciliationsRateLimiting{
		BucketRateLimiting: &akuitytypes.BucketRateLimiting{
			Enabled:    new(true),
			BucketSize: 5,
			BucketQps:  10,
		},
		ItemRateLimiting: &akuitytypes.ItemRateLimiting{
			Enabled:         new(true),
			FailureCooldown: 30,
			BaseDelay:       1,
			MaxDelay:        10,
			BackoffFactor:   1.5,
		},
	}

	ArgocdAppReconciliationsRateLimiting = &argocdv1.AppReconciliationsRateLimiting{
		BucketRateLimiting: &argocdv1.BucketRateLimiting{
			Enabled:    true,
			BucketSize: 5,
			BucketQps:  10,
		},
		ItemRateLimiting: &argocdv1.ItemRateLimiting{
			Enabled:         true,
			FailureCooldown: 30,
			BaseDelay:       1,
			MaxDelay:        10,
			BackoffFactor:   1.5,
		},
	}

	CrossplaneAppReconciliationsRateLimiting = &crossplanetypes.AppReconciliationsRateLimiting{
		BucketRateLimiting: &crossplanetypes.BucketRateLimiting{
			Enabled:    new(true),
			BucketSize: 5,
			BucketQps:  10,
		},
		ItemRateLimiting: &crossplanetypes.ItemRateLimiting{
			Enabled:         new(true),
			FailureCooldown: 30,
			BaseDelay:       1,
			MaxDelay:        10,
			BackoffFactor:   "1.5",
		},
	}

	AkuityInstanceSpec = akuitytypes.InstanceSpec{
		IpAllowList:                     AkuityIpAllowList,
		Subdomain:                       "example.com",
		DeclarativeManagementEnabled:    new(true),
		Extensions:                      AkuityInstallEntryList,
		ClusterCustomizationDefaults:    AkuityClusterCustomization,
		ImageUpdaterEnabled:             new(true),
		BackendIpAllowListEnabled:       new(true),
		RepoServerDelegate:              AkuityRepoServerDelegate,
		AuditExtensionEnabled:           new(true),
		SyncHistoryExtensionEnabled:     new(true),
		ImageUpdaterDelegate:            AkuityImageUpdaterDelegate,
		AppSetDelegate:                  AkuityAppSetDelegate,
		AppsetPolicy:                    AkuityAppsetPolicy,
		HostAliases:                     AkuityHostAliasesList,
		Fqdn:                            "",
		MultiClusterK8SDashboardEnabled: new(true),
		AkuityIntelligenceExtension:     AkuityIntelligenceExtension,
		ImageUpdaterVersion:             "",
		CustomDeprecatedApis:            AkuityCustomDeprecatedApis,
		KubeVisionConfig:                AkuityKubeVisionConfig,
		AppInAnyNamespaceConfig:         AkuityAppInAnyNamespaceConfig,
		Basepath:                        "",
		AppsetProgressiveSyncsEnabled:   new(true),
		AppsetPlugins:                   AkuityAppsetPlugins,
		ApplicationSetExtension:         AkuityApplicationSetExtension,
		AppReconciliationsRateLimiting:  AkuityAppReconciliationsRateLimiting,
	}

	ArgocdInstanceSpec = &argocdv1.InstanceSpec{
		ClusterCustomizationDefaults:    ArgocdClusterCustomization,
		IpAllowList:                     ArgocdIpAllowList,
		Subdomain:                       "example.com",
		DeclarativeManagementEnabled:    true,
		Extensions:                      ArgocdInstallEntryList,
		ImageUpdaterEnabled:             true,
		BackendIpAllowListEnabled:       true,
		RepoServerDelegate:              ArgocdRepoServerDelegate,
		AuditExtensionEnabled:           true,
		SyncHistoryExtensionEnabled:     true,
		ImageUpdaterDelegate:            ArgocdImageUpdaterDelegate,
		AppSetDelegate:                  ArgocdAppSetDelegate,
		AppsetPolicy:                    ArgocdAppsetPolicy,
		HostAliases:                     ArgocdHostAliasesList,
		MultiClusterK8SDashboardEnabled: true,
		AkuityIntelligenceExtension:     ArgocdIntelligenceExtension,
		ImageUpdaterVersion:             "",
		CustomDeprecatedApis:            ArgocdCustomDeprecatedApis,
		KubeVisionConfig:                ArgocdKubeVisionConfig,
		AppInAnyNamespaceConfig:         ArgocdAppInAnyNamespaceConfig,
		Basepath:                        "",
		AppsetProgressiveSyncsEnabled:   true,
		AppsetPlugins:                   ArgocdAppsetPlugins,
		ApplicationSetExtension:         ArgocdApplicationSetExtension,
		AppReconciliationsRateLimiting:  ArgocdAppReconciliationsRateLimiting,
	}

	trueValue              = true
	CrossplaneInstanceSpec = crossplanetypes.InstanceSpec{
		ClusterCustomizationDefaults:    CrossplaneClusterCustomization,
		IpAllowList:                     CrossplaneIpAllowList,
		Subdomain:                       "example.com",
		DeclarativeManagementEnabled:    new(true),
		Extensions:                      CrossplaneInstallEntryList,
		ImageUpdaterEnabled:             new(true),
		BackendIpAllowListEnabled:       new(true),
		RepoServerDelegate:              CrossplaneRepoServerDelegate,
		AuditExtensionEnabled:           new(true),
		SyncHistoryExtensionEnabled:     new(true),
		ImageUpdaterDelegate:            CrossplaneImageUpdateDelegate,
		AppSetDelegate:                  CrossplaneAppSetDelegate,
		AppsetPolicy:                    CrossplaneAppsetPolicy,
		HostAliases:                     CrossplaneHostAliasesList,
		MultiClusterK8SDashboardEnabled: &trueValue,
		AkuityIntelligenceExtension:     CrossplaneIntelligenceExtension,
		ImageUpdaterVersion:             "",
		CustomDeprecatedApis:            CrossplaneCustomDeprecatedApis,
		KubeVisionConfig:                CrossplaneKubeVisionConfig,
		AppInAnyNamespaceConfig:         CrossplaneAppInAnyNamespaceConfig,
		Basepath:                        "",
		AppsetProgressiveSyncsEnabled:   new(true),
		AppsetPlugins:                   CrossplaneAppsetPlugins,
		ApplicationSetExtension:         CrossplaneApplicationSetExtension,
		AppReconciliationsRateLimiting:  CrossplaneAppReconciliationsRateLimiting,
	}

	AkuityInstance = &argocdv1.Instance{
		Name:        InstanceName,
		Description: "test-description",
		Version:     "test-version",
		Spec:        ArgocdInstanceSpec,
	}

	CrossplaneInstance = crossplanetypes.ArgoCD{
		Spec: crossplanetypes.ArgoCDSpec{
			Description:  "test-description",
			Version:      "test-version",
			InstanceSpec: CrossplaneInstanceSpec,
		},
	}

	CrossplaneManagedInstance = v1alpha1.Instance{
		Spec: v1alpha1.InstanceSpec{
			ForProvider: v1alpha1.InstanceParameters{
				Name:                           InstanceName,
				ArgoCD:                         &CrossplaneInstance,
				ArgoCDConfigMap:                nil,
				ArgoCDImageUpdaterConfigMap:    nil,
				ArgoCDImageUpdaterSSHConfigMap: nil,
				ArgoCDNotificationsConfigMap:   nil,
				ArgoCDRBACConfigMap:            nil,
				ArgoCDSSHKnownHostsConfigMap:   nil,
				ArgoCDTLSCertsConfigMap:        nil,
				ConfigManagementPlugins:        nil,
			},
		},
	}
)
