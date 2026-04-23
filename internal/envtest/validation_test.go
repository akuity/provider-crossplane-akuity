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

// Envtest coverage for the parent-reference XOR rule and the top-level
// immutability rules that every MR ships. These are the rules most
// likely to regress on a schema regen (they live on hand-authored
// ForProvider types) and most likely to be silently bypassed by a
// client that skips the CRD — the apiserver is the only reliable gate,
// so the tests run against a real kube-apiserver.

package envtest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// minimalArgoCD returns an ArgoCD blob that satisfies the CRD's
// required-fields schema (spec.version) so Instance creation only ever
// fails on the rule under test.
func minimalArgoCD() *crossplanetypes.ArgoCD {
	return &crossplanetypes.ArgoCD{
		Spec: crossplanetypes.ArgoCDSpec{Version: "v3.1.0"},
	}
}

// minimalKargoSpec returns a KargoSpec with just enough fields set to
// clear the generated type's required/no-omitempty tags.
func minimalKargoSpec() crossplanetypes.KargoSpec {
	return crossplanetypes.KargoSpec{
		Description: "envtest",
		Version:     "v1.4.0",
	}
}

// TestInstanceIpAllowList_ValidatesIDOrRefRequired covers the CEL rule
// on InstanceIpAllowListParameters: exactly one of instanceId or
// instanceRef must be present.
func TestInstanceIpAllowList_ValidatesIDOrRefRequired(t *testing.T) {
	ctx := context.Background()

	missing := &v1alpha1.InstanceIpAllowList{
		ObjectMeta: metav1.ObjectMeta{Name: "ipa-missing"},
		Spec: v1alpha1.InstanceIpAllowListSpec{
			ForProvider: v1alpha1.InstanceIpAllowListParameters{
				AllowList: []*crossplanetypes.IPAllowListEntry{{Ip: "10.0.0.1"}},
			},
		},
	}
	err := kube.Create(ctx, missing)
	require.Error(t, err, "apiserver must reject InstanceIpAllowList missing id+ref")
	assert.Contains(t, err.Error(), "exactly one of instanceId or instanceRef must be set")

	both := &v1alpha1.InstanceIpAllowList{
		ObjectMeta: metav1.ObjectMeta{Name: "ipa-both"},
		Spec: v1alpha1.InstanceIpAllowListSpec{
			ForProvider: v1alpha1.InstanceIpAllowListParameters{
				InstanceID:  "inst-abc",
				InstanceRef: &v1alpha1.LocalReference{Name: "my-instance"},
				AllowList:   []*crossplanetypes.IPAllowListEntry{{Ip: "10.0.0.1"}},
			},
		},
	}
	err = kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject InstanceIpAllowList with both id and ref")
	assert.Contains(t, err.Error(), "exactly one of instanceId or instanceRef must be set")

	withID := &v1alpha1.InstanceIpAllowList{
		ObjectMeta: metav1.ObjectMeta{Name: "ipa-id"},
		Spec: v1alpha1.InstanceIpAllowListSpec{
			ForProvider: v1alpha1.InstanceIpAllowListParameters{
				InstanceID: "inst-abc",
				AllowList:  []*crossplanetypes.IPAllowListEntry{{Ip: "10.0.0.1"}},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withID), "instanceId alone must satisfy CEL")
	t.Cleanup(func() { _ = kube.Delete(ctx, withID) })

	withRef := &v1alpha1.InstanceIpAllowList{
		ObjectMeta: metav1.ObjectMeta{Name: "ipa-ref"},
		Spec: v1alpha1.InstanceIpAllowListSpec{
			ForProvider: v1alpha1.InstanceIpAllowListParameters{
				InstanceRef: &v1alpha1.LocalReference{Name: "my-instance"},
				AllowList:   []*crossplanetypes.IPAllowListEntry{{Ip: "10.0.0.1"}},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withRef), "instanceRef alone must satisfy CEL")
	t.Cleanup(func() { _ = kube.Delete(ctx, withRef) })
}

// TestKargoDefaultShardAgent_ValidatesIDOrRefRequired is the Kargo-side
// mirror — same XOR rule on kargoInstanceId / kargoInstanceRef.
func TestKargoDefaultShardAgent_ValidatesIDOrRefRequired(t *testing.T) {
	ctx := context.Background()

	missing := &v1alpha1.KargoDefaultShardAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "dsa-missing"},
		Spec: v1alpha1.KargoDefaultShardAgentSpec{
			ForProvider: v1alpha1.KargoDefaultShardAgentParameters{AgentName: "shard-a"},
		},
	}
	err := kube.Create(ctx, missing)
	require.Error(t, err, "apiserver must reject KargoDefaultShardAgent missing id+ref")
	assert.Contains(t, err.Error(), "exactly one of kargoInstanceId or kargoInstanceRef must be set")

	both := &v1alpha1.KargoDefaultShardAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "dsa-both"},
		Spec: v1alpha1.KargoDefaultShardAgentSpec{
			ForProvider: v1alpha1.KargoDefaultShardAgentParameters{
				KargoInstanceID:  "ki-abc",
				KargoInstanceRef: &v1alpha1.LocalReference{Name: "my-kargo"},
				AgentName:        "shard-a",
			},
		},
	}
	err = kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject KargoDefaultShardAgent with both id and ref")
	assert.Contains(t, err.Error(), "exactly one of kargoInstanceId or kargoInstanceRef must be set")

	withID := &v1alpha1.KargoDefaultShardAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "dsa-id"},
		Spec: v1alpha1.KargoDefaultShardAgentSpec{
			ForProvider: v1alpha1.KargoDefaultShardAgentParameters{
				KargoInstanceID: "ki-abc",
				AgentName:       "shard-a",
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withID))
	t.Cleanup(func() { _ = kube.Delete(ctx, withID) })
}

// TestCluster_ValidatesIDOrRefRequired covers the XOR on
// ClusterParameters.
func TestCluster_ValidatesIDOrRefRequired(t *testing.T) {
	ctx := context.Background()

	missing := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-missing"},
		Spec: v1alpha1.ClusterSpec{
			ForProvider: v1alpha1.ClusterParameters{Name: "c1"},
		},
	}
	err := kube.Create(ctx, missing)
	require.Error(t, err, "apiserver must reject Cluster missing id+ref")
	assert.Contains(t, err.Error(), "exactly one of instanceId or instanceRef must be set")

	both := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-both"},
		Spec: v1alpha1.ClusterSpec{
			ForProvider: v1alpha1.ClusterParameters{
				InstanceID:  "inst-abc",
				InstanceRef: &v1alpha1.LocalReference{Name: "my-instance"},
				Name:        "c1",
			},
		},
	}
	err = kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject Cluster with both id and ref")
	assert.Contains(t, err.Error(), "exactly one of instanceId or instanceRef must be set")

	withID := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-id"},
		Spec: v1alpha1.ClusterSpec{
			ForProvider: v1alpha1.ClusterParameters{InstanceID: "inst-abc", Name: "c1"},
		},
	}
	require.NoError(t, kube.Create(ctx, withID))
	t.Cleanup(func() { _ = kube.Delete(ctx, withID) })
}

// TestKargoAgent_ValidatesIDOrRefRequired covers the XOR on
// KargoAgentParameters.
func TestKargoAgent_ValidatesIDOrRefRequired(t *testing.T) {
	ctx := context.Background()

	missing := &v1alpha1.KargoAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "ka-missing"},
		Spec: v1alpha1.KargoAgentSpec{
			ForProvider: v1alpha1.KargoAgentParameters{Name: "agent-a"},
		},
	}
	err := kube.Create(ctx, missing)
	require.Error(t, err, "apiserver must reject KargoAgent missing id+ref")
	assert.Contains(t, err.Error(), "exactly one of kargoInstanceId or kargoInstanceRef must be set")

	both := &v1alpha1.KargoAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "ka-both"},
		Spec: v1alpha1.KargoAgentSpec{
			ForProvider: v1alpha1.KargoAgentParameters{
				KargoInstanceID:  "ki-abc",
				KargoInstanceRef: &v1alpha1.LocalReference{Name: "my-kargo"},
				Name:             "agent-a",
			},
		},
	}
	err = kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject KargoAgent with both id and ref")
	assert.Contains(t, err.Error(), "exactly one of kargoInstanceId or kargoInstanceRef must be set")

	withID := &v1alpha1.KargoAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "ka-id"},
		Spec: v1alpha1.KargoAgentSpec{
			ForProvider: v1alpha1.KargoAgentParameters{
				KargoInstanceID: "ki-abc",
				Name:            "agent-a",
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withID))
	t.Cleanup(func() { _ = kube.Delete(ctx, withID) })
}

// TestInstance_NameImmutable covers the name-immutable rule on
// InstanceParameters — Instance has no parent ref, so name immutability
// is the only top-level CEL rule.
func TestInstance_NameImmutable(t *testing.T) {
	ctx := context.Background()

	inst := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-name-immut"},
		Spec: v1alpha1.InstanceSpec{
			ForProvider: v1alpha1.InstanceParameters{
				Name:   "original",
				ArgoCD: minimalArgoCD(),
			},
		},
	}
	require.NoError(t, kube.Create(ctx, inst))
	t.Cleanup(func() { _ = kube.Delete(ctx, inst) })

	got := &v1alpha1.Instance{}
	require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(inst), got))
	got.Spec.ForProvider.Name = "renamed"
	err := kube.Update(ctx, got)
	require.Error(t, err, "apiserver must reject Instance name rename")
	assert.Contains(t, err.Error(), "name is immutable")
}

// TestCluster_NameImmutable covers the name-immutable rule on
// ClusterParameters.
func TestCluster_NameImmutable(t *testing.T) {
	ctx := context.Background()

	cl := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-name-immut"},
		Spec: v1alpha1.ClusterSpec{
			ForProvider: v1alpha1.ClusterParameters{InstanceID: "inst-abc", Name: "c1"},
		},
	}
	require.NoError(t, kube.Create(ctx, cl))
	t.Cleanup(func() { _ = kube.Delete(ctx, cl) })

	got := &v1alpha1.Cluster{}
	require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(cl), got))
	got.Spec.ForProvider.Name = "c2"
	err := kube.Update(ctx, got)
	require.Error(t, err, "apiserver must reject Cluster name rename")
	assert.Contains(t, err.Error(), "name is immutable")
}

// TestKargoInstance_NameImmutable covers the name-immutable rule on
// KargoInstanceParameters.
func TestKargoInstance_NameImmutable(t *testing.T) {
	ctx := context.Background()

	ki := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki-name-immut"},
		Spec: v1alpha1.KargoInstanceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Name:  "kiname",
				Kargo: minimalKargoSpec(),
			},
		},
	}
	require.NoError(t, kube.Create(ctx, ki))
	t.Cleanup(func() { _ = kube.Delete(ctx, ki) })

	got := &v1alpha1.KargoInstance{}
	require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(ki), got))
	got.Spec.ForProvider.Name = "renamed"
	err := kube.Update(ctx, got)
	require.Error(t, err, "apiserver must reject KargoInstance name rename")
	assert.Contains(t, err.Error(), "name is immutable")
}

// TestKargoAgent_NameImmutable covers the name-immutable rule on
// KargoAgentParameters.
func TestKargoAgent_NameImmutable(t *testing.T) {
	ctx := context.Background()

	ka := &v1alpha1.KargoAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "ka-name-immut"},
		Spec: v1alpha1.KargoAgentSpec{
			ForProvider: v1alpha1.KargoAgentParameters{
				KargoInstanceID: "ki-abc",
				Name:            "agent-a",
			},
		},
	}
	require.NoError(t, kube.Create(ctx, ka))
	t.Cleanup(func() { _ = kube.Delete(ctx, ka) })

	got := &v1alpha1.KargoAgent{}
	require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(ka), got))
	got.Spec.ForProvider.Name = "agent-b"
	err := kube.Update(ctx, got)
	require.Error(t, err, "apiserver must reject KargoAgent name rename")
	assert.Contains(t, err.Error(), "name is immutable")
}

// TestCluster_InstanceIDImmutable covers the id/ref-immutable rule on
// ClusterParameters. The rule blocks flipping instanceId between two
// values and flipping the *shape* of the reference (id <-> ref).
func TestCluster_InstanceIDImmutable(t *testing.T) {
	ctx := context.Background()

	cl := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-id-immut"},
		Spec: v1alpha1.ClusterSpec{
			ForProvider: v1alpha1.ClusterParameters{InstanceID: "inst-abc", Name: "c1"},
		},
	}
	require.NoError(t, kube.Create(ctx, cl))
	t.Cleanup(func() { _ = kube.Delete(ctx, cl) })

	// Rename the ID.
	got := &v1alpha1.Cluster{}
	require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(cl), got))
	got.Spec.ForProvider.InstanceID = "inst-xyz"
	err := kube.Update(ctx, got)
	require.Error(t, err, "apiserver must reject instanceId rename")
	assert.Contains(t, err.Error(), "instanceId/instanceRef are immutable")

	// Flip from ID to Ref. Need to reread to avoid resource-version drift.
	require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(cl), got))
	got.Spec.ForProvider.InstanceID = ""
	got.Spec.ForProvider.InstanceRef = &v1alpha1.LocalReference{Name: "some-instance"}
	err = kube.Update(ctx, got)
	require.Error(t, err, "apiserver must reject id->ref flip")
	assert.Contains(t, err.Error(), "instanceId/instanceRef are immutable")
}
