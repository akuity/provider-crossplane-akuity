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

//go:build envtest

package envtest_test

import (
	"context"
	"testing"

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
	assert.Contains(t, err.Error(), "one of instanceId or instanceRef must be set")

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
	assert.Contains(t, err.Error(), "one of kargoInstanceId or kargoInstanceRef must be set")

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
