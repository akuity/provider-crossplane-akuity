// Package convert_test exercises symmetry of the generated v1alpha2 <-> akuity
// wire converters. The goal is not exhaustive coverage — the codegen is
// already locked byte-identically by the golden snapshots in
// internal/types/test/roundtrip/ — but a quick early-warning check that the
// hand-authored AST walker keeps SpecToAPI and APIToSpec inverses of each
// other across the top-level types that controllers care about.
package convert_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert"
)

func TestClusterData_RoundTrip(t *testing.T) {
	in := &v1alpha2.ClusterData{
		Size:                      v1alpha2.ClusterSize("small"),
		AutoUpgradeDisabled:       ptr.To(true),
		Kustomization:             "patchesJson6902:\n- patch: noop\n",
		AppReplication:            ptr.To(false),
		TargetVersion:             "v1.2.3",
		RedisTunneling:            ptr.To(true),
		DatadogAnnotationsEnabled: ptr.To(false),
		EksAddonEnabled:           ptr.To(true),
		MaintenanceMode:           ptr.To(true),
	}

	out := convert.ClusterDataAPIToSpec(convert.ClusterDataSpecToAPI(in))
	if diff := cmp.Diff(in, out); diff != "" {
		t.Errorf("ClusterData round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestArgoCDInstanceSpec_RoundTrip(t *testing.T) {
	in := &v1alpha2.ArgoCDInstanceSpec{
		Subdomain:                    "team-platform",
		DeclarativeManagementEnabled:  ptr.To(true),
		ImageUpdaterEnabled:           ptr.To(true),
		BackendIpAllowListEnabled:     ptr.To(true),
		AuditExtensionEnabled:         ptr.To(false),
		SyncHistoryExtensionEnabled:   ptr.To(true),
		AssistantExtensionEnabled:     ptr.To(false),
	}

	out := convert.InstanceSpecAPIToSpec(convert.InstanceSpecSpecToAPI(in))
	if diff := cmp.Diff(in, out); diff != "" {
		t.Errorf("ArgoCDInstanceSpec round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestKargoSpec_RoundTrip(t *testing.T) {
	in := &v1alpha2.KargoSpec{
		Description: "Platform Kargo",
		Version:     "v1.2.0",
		Fqdn:        "kargo.example.com",
		Subdomain:   "kargo",
		KargoInstanceSpec: v1alpha2.KargoInstanceSpec{
			BackendIpAllowListEnabled:  ptr.To(true),
			DefaultShardAgent:         "default",
			GlobalCredentialsNs:       []string{"platform"},
			GlobalServiceAccountNs:    []string{"platform-sa"},
			PromoControllerEnabled:     ptr.To(true),
		},
	}

	out := convert.KargoSpecAPIToSpec(convert.KargoSpecSpecToAPI(in))
	if diff := cmp.Diff(in, out); diff != "" {
		t.Errorf("KargoSpec round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestKargoInstanceSpec_RoundTrip(t *testing.T) {
	in := &v1alpha2.KargoInstanceSpec{
		BackendIpAllowListEnabled:  ptr.To(true),
		IpAllowList: []*v1alpha2.KargoIPAllowListEntry{
			{Ip: "10.0.0.0/8", Description: "vpc"},
			{Ip: "192.168.0.0/16", Description: "lab"},
		},
		DefaultShardAgent:      "prod",
		GlobalCredentialsNs:    []string{"ns-a", "ns-b"},
		GlobalServiceAccountNs: []string{"sa-a"},
		PromoControllerEnabled:  ptr.To(true),
	}

	out := convert.KargoInstanceSpecAPIToSpec(convert.KargoInstanceSpecSpecToAPI(in))
	if diff := cmp.Diff(in, out); diff != "" {
		t.Errorf("KargoInstanceSpec round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestKargoAgentData_RoundTrip(t *testing.T) {
	in := &v1alpha2.KargoAgentData{
		Size:                 v1alpha2.KargoAgentSize("medium"),
		AutoUpgradeDisabled:  ptr.To(true),
		TargetVersion:        "v1.1.0",
		Kustomization:        "patchesJson6902:\n- patch: noop\n",
		RemoteArgocd:         "https://argocd.example.com",
		AkuityManaged:         ptr.To(true),
		ArgocdNamespace:      "argocd",
		SelfManagedArgocdUrl: "",
	}

	out := convert.KargoAgentDataAPIToSpec(convert.KargoAgentDataSpecToAPI(in))
	if diff := cmp.Diff(in, out); diff != "" {
		t.Errorf("KargoAgentData round-trip mismatch (-want +got):\n%s", diff)
	}
}
