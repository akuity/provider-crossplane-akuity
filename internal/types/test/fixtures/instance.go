package fixtures

import (
	"k8s.io/utils/ptr"

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
		OverridePolicy: true,
	}

	ArgocdAppsetPolicy = &argocdv1.AppsetPolicy{
		Policy:         "policy",
		OverridePolicy: true,
	}

	CrossplaneAppsetPolicy = &crossplanetypes.AppsetPolicy{
		Policy:         "policy",
		OverridePolicy: ptr.To(true),
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
		ControlPlane: true,
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
		ControlPlane: ptr.To(true),
		ManagedCluster: &crossplanetypes.ManagedCluster{
			ClusterName: "test-cluster",
		},
	}

	AkuityRepoServerDelegate = &akuitytypes.RepoServerDelegate{
		ControlPlane: true,
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
		ControlPlane: ptr.To(true),
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
		AutoUpgradeDisabled: true,
		Kustomization:       Kustomization,
		AppReplication:      true,
		RedisTunneling:      true,
	}

	ArgocdClusterCustomization = &argocdv1.ClusterCustomization{
		AutoUpgradeDisabled: true,
		AppReplication:      true,
		RedisTunneling:      true,
		Kustomization:       KustomizationPB,
	}

	CrossplaneClusterCustomization = &crossplanetypes.ClusterCustomization{
		AutoUpgradeDisabled: ptr.To(true),
		Kustomization:       KustomizationYAML,
		AppReplication:      ptr.To(true),
		RedisTunneling:      ptr.To(true),
	}

	AkuityInstanceSpec = akuitytypes.InstanceSpec{
		IpAllowList:                     AkuityIpAllowList,
		Subdomain:                       "example.com",
		DeclarativeManagementEnabled:    true,
		Extensions:                      AkuityInstallEntryList,
		ClusterCustomizationDefaults:    AkuityClusterCustomization,
		ImageUpdaterEnabled:             true,
		BackendIpAllowListEnabled:       true,
		RepoServerDelegate:              AkuityRepoServerDelegate,
		AuditExtensionEnabled:           true,
		SyncHistoryExtensionEnabled:     true,
		ImageUpdaterDelegate:            AkuityImageUpdaterDelegate,
		AppSetDelegate:                  AkuityAppSetDelegate,
		AssistantExtensionEnabled:       true,
		AppsetPolicy:                    AkuityAppsetPolicy,
		HostAliases:                     AkuityHostAliasesList,
		Fqdn:                            "",
		MultiClusterK8SDashboardEnabled: true,
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
		AssistantExtensionEnabled:       true,
		AppsetPolicy:                    ArgocdAppsetPolicy,
		HostAliases:                     ArgocdHostAliasesList,
		MultiClusterK8SDashboardEnabled: true,
	}

	trueValue              = true
	CrossplaneInstanceSpec = crossplanetypes.InstanceSpec{
		ClusterCustomizationDefaults:    CrossplaneClusterCustomization,
		IpAllowList:                     CrossplaneIpAllowList,
		Subdomain:                       "example.com",
		DeclarativeManagementEnabled:    ptr.To(true),
		Extensions:                      CrossplaneInstallEntryList,
		ImageUpdaterEnabled:             ptr.To(true),
		BackendIpAllowListEnabled:       ptr.To(true),
		RepoServerDelegate:              CrossplaneRepoServerDelegate,
		AuditExtensionEnabled:           ptr.To(true),
		SyncHistoryExtensionEnabled:     ptr.To(true),
		ImageUpdaterDelegate:            CrossplaneImageUpdateDelegate,
		AppSetDelegate:                  CrossplaneAppSetDelegate,
		AssistantExtensionEnabled:       ptr.To(true),
		AppsetPolicy:                    CrossplaneAppsetPolicy,
		HostAliases:                     CrossplaneHostAliasesList,
		MultiClusterK8SDashboardEnabled: &trueValue,
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
