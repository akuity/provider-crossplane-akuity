//go:build envtest

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

package envtest_test

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
)

// Namespace used by every validation test. envtest keeps etcd state
// across tests in a single process, so a fresh NS per test isn't needed
// — object names are already scoped per-test.
const validationNS = "default"

// newMeta stamps a resource with the shared validation-test namespace.
func newMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: validationNS}
}

// TestInstanceIpAllowList_ValidatesIDOrRefRequired exercises the CEL
// rule on InstanceIpAllowListParameters: one of instanceId or
// instanceRef must be present. The apiserver (not the controller) must
// reject a manifest that sets neither.
func TestInstanceIpAllowList_ValidatesIDOrRefRequired(t *testing.T) {
	ctx := context.Background()

	missing := &corev1alpha2.InstanceIpAllowList{
		ObjectMeta: newMeta("ipa-missing"),
		Spec: corev1alpha2.InstanceIpAllowListSpec{
			ForProvider: corev1alpha2.InstanceIpAllowListParameters{
				AllowList: []*corev1alpha2.IPAllowListEntry{{Ip: "10.0.0.1"}},
			},
		},
	}
	err := kube.Create(ctx, missing)
	require.Error(t, err, "apiserver must reject InstanceIpAllowList missing id+ref")
	assert.Contains(t, err.Error(), "exactly one of instanceId or instanceRef must be set")

	both := &corev1alpha2.InstanceIpAllowList{
		ObjectMeta: newMeta("ipa-both"),
		Spec: corev1alpha2.InstanceIpAllowListSpec{
			ForProvider: corev1alpha2.InstanceIpAllowListParameters{
				InstanceID:  "inst-abc",
				InstanceRef: &corev1alpha2.LocalReference{Name: "my-instance"},
				AllowList:   []*corev1alpha2.IPAllowListEntry{{Ip: "10.0.0.1"}},
			},
		},
	}
	err = kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject InstanceIpAllowList with both id and ref")
	assert.Contains(t, err.Error(), "exactly one of instanceId or instanceRef must be set")

	withID := &corev1alpha2.InstanceIpAllowList{
		ObjectMeta: newMeta("ipa-id"),
		Spec: corev1alpha2.InstanceIpAllowListSpec{
			ForProvider: corev1alpha2.InstanceIpAllowListParameters{
				InstanceID: "inst-abc",
				AllowList:  []*corev1alpha2.IPAllowListEntry{{Ip: "10.0.0.1"}},
			},
			// ManagedResourceSpec.ProviderConfigReference has a
			// kubebuilder default (ClusterProviderConfig/default) so we
			// don't need to set it here. The controller isn't running
			// during this test, so the ref never resolves.
		},
	}
	require.NoError(t, kube.Create(ctx, withID), "instanceId alone must satisfy CEL")
	t.Cleanup(func() { _ = kube.Delete(ctx, withID) })

	withRef := &corev1alpha2.InstanceIpAllowList{
		ObjectMeta: newMeta("ipa-ref"),
		Spec: corev1alpha2.InstanceIpAllowListSpec{
			ForProvider: corev1alpha2.InstanceIpAllowListParameters{
				InstanceRef: &corev1alpha2.LocalReference{Name: "my-instance"},
				AllowList:   []*corev1alpha2.IPAllowListEntry{{Ip: "10.0.0.1"}},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withRef), "instanceRef alone must satisfy CEL")
	t.Cleanup(func() { _ = kube.Delete(ctx, withRef) })
}

// TestKargoDefaultShardAgent_ValidatesIDOrRefRequired is the Kargo-side
// mirror — same CEL rule on kargoInstanceId / kargoInstanceRef.
func TestKargoDefaultShardAgent_ValidatesIDOrRefRequired(t *testing.T) {
	ctx := context.Background()

	missing := &corev1alpha2.KargoDefaultShardAgent{
		ObjectMeta: newMeta("dsa-missing"),
		Spec: corev1alpha2.KargoDefaultShardAgentSpec{
			ForProvider: corev1alpha2.KargoDefaultShardAgentParameters{
				AgentName: "shard-a",
			},
		},
	}
	err := kube.Create(ctx, missing)
	require.Error(t, err, "apiserver must reject KargoDefaultShardAgent missing id+ref")
	assert.Contains(t, err.Error(), "exactly one of kargoInstanceId or kargoInstanceRef must be set")

	both := &corev1alpha2.KargoDefaultShardAgent{
		ObjectMeta: newMeta("dsa-both"),
		Spec: corev1alpha2.KargoDefaultShardAgentSpec{
			ForProvider: corev1alpha2.KargoDefaultShardAgentParameters{
				KargoInstanceID:  "ki-abc",
				KargoInstanceRef: &corev1alpha2.LocalReference{Name: "my-kargo"},
				AgentName:        "shard-a",
			},
		},
	}
	err = kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject KargoDefaultShardAgent with both id and ref")
	assert.Contains(t, err.Error(), "exactly one of kargoInstanceId or kargoInstanceRef must be set")

	withID := &corev1alpha2.KargoDefaultShardAgent{
		ObjectMeta: newMeta("dsa-id"),
		Spec: corev1alpha2.KargoDefaultShardAgentSpec{
			ForProvider: corev1alpha2.KargoDefaultShardAgentParameters{
				KargoInstanceID: "ki-abc",
				AgentName:       "shard-a",
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withID))
	t.Cleanup(func() { _ = kube.Delete(ctx, withID) })
}

// TestCluster_ValidatesIDOrRefRequired exercises the CEL rule on
// ClusterParameters: exactly one of instanceId or instanceRef must be
// set.
func TestCluster_ValidatesIDOrRefRequired(t *testing.T) {
	ctx := context.Background()

	missing := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cluster-missing"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				Name: "c1",
			},
		},
	}
	err := kube.Create(ctx, missing)
	require.Error(t, err, "apiserver must reject Cluster missing id+ref")
	assert.Contains(t, err.Error(), "exactly one of instanceId or instanceRef must be set")

	both := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cluster-both"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID:  "inst-abc",
				InstanceRef: &corev1alpha2.LocalReference{Name: "my-instance"},
				Name:        "c1",
			},
		},
	}
	err = kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject Cluster with both id and ref")
	assert.Contains(t, err.Error(), "exactly one of instanceId or instanceRef must be set")

	withID := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cluster-id"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID: "inst-abc",
				Name:       "c1",
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withID))
	t.Cleanup(func() { _ = kube.Delete(ctx, withID) })
}

// TestCluster_KubeConfigSourceMutualExclusion exercises the CEL rule
// that forbids setting both kubeconfigSecretRef and
// enableInClusterKubeconfig on the same Cluster.
func TestCluster_KubeConfigSourceMutualExclusion(t *testing.T) {
	ctx := context.Background()

	both := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cluster-kc-both"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID:                "inst-abc",
				Name:                      "c1",
				KubeConfigSecretRef:       &xpv1.LocalSecretReference{Name: "kc"},
				EnableInClusterKubeConfig: true,
			},
		},
	}
	err := kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject Cluster with both kubeconfig sources")
	assert.Contains(t, err.Error(), "kubeconfigSecretRef and enableInClusterKubeconfig are mutually exclusive")

	withRef := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cluster-kc-ref"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID:          "inst-abc",
				Name:                "c1",
				KubeConfigSecretRef: &xpv1.LocalSecretReference{Name: "kc"},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withRef))
	t.Cleanup(func() { _ = kube.Delete(ctx, withRef) })

	withInCluster := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cluster-kc-local"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID:                "inst-abc",
				Name:                      "c1",
				EnableInClusterKubeConfig: true,
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withInCluster))
	t.Cleanup(func() { _ = kube.Delete(ctx, withInCluster) })
}

// TestCluster_RemoveAgentRequiresKubeConfigSource exercises the CEL
// rule that gates removeAgentResourcesOnDestroy on one of the two
// kubeconfig sources being set — otherwise the controller has no way
// to reach the managed cluster to clean anything up.
func TestCluster_RemoveAgentRequiresKubeConfigSource(t *testing.T) {
	ctx := context.Background()

	orphan := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cluster-rm-orphan"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID:                    "inst-abc",
				Name:                          "c1",
				RemoveAgentResourcesOnDestroy: true,
			},
		},
	}
	err := kube.Create(ctx, orphan)
	require.Error(t, err, "apiserver must reject removeAgentResourcesOnDestroy without a kubeconfig source")
	assert.Contains(t, err.Error(), "removeAgentResourcesOnDestroy requires kubeconfigSecretRef or enableInClusterKubeconfig")

	ok := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cluster-rm-ok"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID:                    "inst-abc",
				Name:                          "c1",
				EnableInClusterKubeConfig:     true,
				RemoveAgentResourcesOnDestroy: true,
			},
		},
	}
	require.NoError(t, kube.Create(ctx, ok))
	t.Cleanup(func() { _ = kube.Delete(ctx, ok) })
}

// TestKargoAgent_ValidatesIDOrRefRequired exercises the CEL rule on
// KargoAgentParameters: exactly one of kargoInstanceId or
// kargoInstanceRef must be set.
func TestKargoAgent_ValidatesIDOrRefRequired(t *testing.T) {
	ctx := context.Background()

	missing := &corev1alpha2.KargoAgent{
		ObjectMeta: newMeta("ka-missing"),
		Spec: corev1alpha2.KargoAgentResourceSpec{
			ForProvider: corev1alpha2.KargoAgentParameters{
				Name: "agent-a",
			},
		},
	}
	err := kube.Create(ctx, missing)
	require.Error(t, err, "apiserver must reject KargoAgent missing id+ref")
	assert.Contains(t, err.Error(), "exactly one of kargoInstanceId or kargoInstanceRef must be set")

	both := &corev1alpha2.KargoAgent{
		ObjectMeta: newMeta("ka-both"),
		Spec: corev1alpha2.KargoAgentResourceSpec{
			ForProvider: corev1alpha2.KargoAgentParameters{
				KargoInstanceID:  "ki-abc",
				KargoInstanceRef: &corev1alpha2.LocalReference{Name: "my-kargo"},
				Name:             "agent-a",
			},
		},
	}
	err = kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject KargoAgent with both id and ref")
	assert.Contains(t, err.Error(), "exactly one of kargoInstanceId or kargoInstanceRef must be set")

	withID := &corev1alpha2.KargoAgent{
		ObjectMeta: newMeta("ka-id"),
		Spec: corev1alpha2.KargoAgentResourceSpec{
			ForProvider: corev1alpha2.KargoAgentParameters{
				KargoInstanceID: "ki-abc",
				Name:            "agent-a",
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withID))
	t.Cleanup(func() { _ = kube.Delete(ctx, withID) })
}
