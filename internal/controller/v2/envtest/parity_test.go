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

// Envtest coverage for the parity additions (Phases 1–4): secret-ref
// shapes + repo-* CEL, dex secret mutual-exclusion, ClusterData
// maintenance/size rules, and preserve-unknown-fields round-tripping
// of argocdResources / kargoResources. Reconcile-level behavior
// (rotation detection, controller wiring) is covered by the fake-kube
// unit tests in the controller packages; envtest here locks in the
// apiserver-enforced pieces that unit tests can't observe.

package envtest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
)

func mustRawExt(t *testing.T, obj map[string]interface{}) runtime.RawExtension {
	t.Helper()
	b, err := json.Marshal(obj)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: b}
}

// TestInstance_RepoCredentialNamesMustStartWithRepoPrefix exercises the
// CEL rule guarding spec.forProvider.repoCredentialSecretRefs[*].name —
// each entry's Name is the gateway-facing slot, which must carry the
// "repo-" prefix so ArgoCD recognises it as a scoped credential. A
// mistyped Name is a silent footgun at the API layer, so the CRD
// rejects it at admission.
func TestInstance_RepoCredentialNamesMustStartWithRepoPrefix(t *testing.T) {
	ctx := context.Background()

	bad := &corev1alpha2.Instance{
		ObjectMeta: newMeta("inst-bad-repo-name"),
		Spec: corev1alpha2.InstanceSpec{
			ForProvider: corev1alpha2.InstanceParameters{
				Name:   "inst-bad-repo-name",
				ArgoCD: &corev1alpha2.ArgoCDSpec{Version: "v3.1.0"},
				RepoCredentialSecretRefs: []corev1alpha2.NamedLocalSecretReference{{
					Name:      "github-prod", // missing repo- prefix
					SecretRef: xpv1.LocalSecretReference{Name: "gh-prod"},
				}},
			},
		},
	}
	err := kube.Create(ctx, bad)
	require.Error(t, err, "apiserver must reject repoCredentialSecretRefs entry without repo- prefix")
	assert.Contains(t, err.Error(), "repo-")

	badTmpl := &corev1alpha2.Instance{
		ObjectMeta: newMeta("inst-bad-repo-tmpl"),
		Spec: corev1alpha2.InstanceSpec{
			ForProvider: corev1alpha2.InstanceParameters{
				Name:   "inst-bad-repo-tmpl",
				ArgoCD: &corev1alpha2.ArgoCDSpec{Version: "v3.1.0"},
				RepoTemplateCredentialSecretRefs: []corev1alpha2.NamedLocalSecretReference{{
					Name:      "tmpl",
					SecretRef: xpv1.LocalSecretReference{Name: "tmpl"},
				}},
			},
		},
	}
	err = kube.Create(ctx, badTmpl)
	require.Error(t, err, "apiserver must reject repoTemplateCredentialSecretRefs entry without repo- prefix")
	assert.Contains(t, err.Error(), "repo-")

	good := &corev1alpha2.Instance{
		ObjectMeta: newMeta("inst-good-repo-name"),
		Spec: corev1alpha2.InstanceSpec{
			ForProvider: corev1alpha2.InstanceParameters{
				Name:   "inst-good-repo-name",
				ArgoCD: &corev1alpha2.ArgoCDSpec{Version: "v3.1.0"},
				RepoCredentialSecretRefs: []corev1alpha2.NamedLocalSecretReference{{
					Name:      "repo-github-prod",
					SecretRef: xpv1.LocalSecretReference{Name: "gh-prod"},
				}},
				RepoTemplateCredentialSecretRefs: []corev1alpha2.NamedLocalSecretReference{{
					Name:      "repo-templates",
					SecretRef: xpv1.LocalSecretReference{Name: "tmpl"},
				}},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, good), "repo- prefixed names must satisfy CEL")
	t.Cleanup(func() { _ = kube.Delete(ctx, good) })
}

// TestInstance_RepoCredentialNameRegexRejectsBypasses covers bypasses
// the old bare-prefix CEL rule ('startsWith("repo-")') accepted:
// whitespace, uppercase, and the degenerate "repo-" alone. The
// tightened regex (^repo-[a-z0-9][a-z0-9-]*$) should reject all of
// them at admission so malformed gateway slot names never hit Apply.
func TestInstance_RepoCredentialNameRegexRejectsBypasses(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		desc string
		name string
	}{
		{"trailing space", "repo- "},
		{"uppercase", "repo-Foo"},
		{"prefix only", "repo-"},
		{"starts with dash", "repo--foo"},
		{"unicode", "repo-héllo"},
	}
	for i, tc := range cases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			inst := &corev1alpha2.Instance{
				ObjectMeta: newMeta(fmt.Sprintf("inst-regex-%d", i)),
				Spec: corev1alpha2.InstanceSpec{
					ForProvider: corev1alpha2.InstanceParameters{
						Name:   fmt.Sprintf("inst-regex-%d", i),
						ArgoCD: &corev1alpha2.ArgoCDSpec{Version: "v3.1.0"},
						RepoCredentialSecretRefs: []corev1alpha2.NamedLocalSecretReference{{
							Name:      tc.name,
							SecretRef: xpv1.LocalSecretReference{Name: "x"},
						}},
					},
				},
			}
			err := kube.Create(ctx, inst)
			require.Error(t, err, "regex must reject %q", tc.name)
		})
	}
}

// TestInstance_ArgocdResourcesRoundTrip verifies that the
// preserve-unknown-fields schema on argocdResources carries opaque
// manifests through the apiserver unchanged. The controller is not
// running in this suite, so we assert persistence only — routing /
// kind validation lives in the unit tests for splitArgocdResources.
func TestInstance_ArgocdResourcesRoundTrip(t *testing.T) {
	ctx := context.Background()

	inst := &corev1alpha2.Instance{
		ObjectMeta: newMeta("inst-argocd-resources"),
		Spec: corev1alpha2.InstanceSpec{
			ForProvider: corev1alpha2.InstanceParameters{
				Name:   "inst-argocd-resources",
				ArgoCD: &corev1alpha2.ArgoCDSpec{Version: "v3.1.0"},
				ArgocdResources: []runtime.RawExtension{
					mustRawExt(t, map[string]interface{}{
						"apiVersion": "argoproj.io/v1alpha1",
						"kind":       "AppProject",
						"metadata":   map[string]interface{}{"name": "team-a"},
						"spec": map[string]interface{}{
							"description": "team a",
						},
					}),
				},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, inst))
	t.Cleanup(func() { _ = kube.Delete(ctx, inst) })

	got := &corev1alpha2.Instance{}
	require.NoError(t, kube.Get(ctx, client.ObjectKey{Namespace: inst.Namespace, Name: inst.Name}, got))
	require.Len(t, got.Spec.ForProvider.Resources, 1)

	// Decoded payload must still carry the allowlisted apiVersion/kind,
	// independent of whether kube-apiserver reserialized the bytes.
	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(got.Spec.ForProvider.Resources[0].Raw, &obj))
	assert.Equal(t, "argoproj.io/v1alpha1", obj["apiVersion"])
	assert.Equal(t, "AppProject", obj["kind"])
}

// TestKargoInstance_DexMutualExclusion locks in the CEL rule that
// forbids setting both the wire-shape inline spec.oidcConfig.dexConfigSecret
// and the hand-authored top-level DexConfigSecretRef. The rule lives on
// KargoInstanceParameters (hand-authored) rather than on the generated
// KargoOidcConfig leaf so a gencrossplanetypes regen can't drop it.
func TestKargoInstance_DexMutualExclusion(t *testing.T) {
	ctx := context.Background()

	// The CRD marks every OIDC leaf `+optional`, so ref-only and
	// inline-only manifests round-trip without zero-value padding.
	// Guards against regressions in the wire-file `+optional`
	// markers that would re-introduce admission-hostile required
	// fields.
	v := "inline-secret"
	both := &corev1alpha2.KargoInstance{
		ObjectMeta: newMeta("ki-dex-both"),
		Spec: corev1alpha2.KargoInstanceResourceSpec{
			ForProvider: corev1alpha2.KargoInstanceParameters{
				Name: "ki-dex-both",
				Kargo: corev1alpha2.KargoSpec{
					Version: "v1.4.0",
					OidcConfig: &corev1alpha2.KargoOidcConfig{
						DexConfigSecret: map[string]corev1alpha2.Value{
							"client-secret": {Value: &v},
						},
						DexConfigSecretRef: &xpv1.LocalSecretReference{Name: "dex-creds"},
					},
				},
			},
		},
	}
	err := kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject both dexConfigSecret and dexConfigSecretRef")
	assert.Contains(t, err.Error(), "set either spec.oidcConfig.dexConfigSecretRef or spec.oidcConfig.dexConfigSecret")

	onlyRef := &corev1alpha2.KargoInstance{
		ObjectMeta: newMeta("ki-dex-ref-only"),
		Spec: corev1alpha2.KargoInstanceResourceSpec{
			ForProvider: corev1alpha2.KargoInstanceParameters{
				Name: "ki-dex-ref-only",
				Kargo: corev1alpha2.KargoSpec{
					Version: "v1.4.0",
					OidcConfig: &corev1alpha2.KargoOidcConfig{
						DexConfigSecretRef: &xpv1.LocalSecretReference{Name: "dex-creds"},
					},
				},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, onlyRef), "dexConfigSecretRef alone must satisfy CRD + CEL")
	t.Cleanup(func() { _ = kube.Delete(ctx, onlyRef) })

	onlyInline := &corev1alpha2.KargoInstance{
		ObjectMeta: newMeta("ki-dex-inline-only"),
		Spec: corev1alpha2.KargoInstanceResourceSpec{
			ForProvider: corev1alpha2.KargoInstanceParameters{
				Name: "ki-dex-inline-only",
				Kargo: corev1alpha2.KargoSpec{
					Version: "v1.4.0",
					OidcConfig: &corev1alpha2.KargoOidcConfig{
						DexConfigSecret: map[string]corev1alpha2.Value{
							"client-secret": {Value: &v},
						},
					},
				},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, onlyInline), "inline-only must still work for soft migration")
	t.Cleanup(func() { _ = kube.Delete(ctx, onlyInline) })
}

// TestKargoInstance_KargoResourcesRoundTrip locks in that the
// preserve-unknown-fields schema on kargoResources carries opaque
// manifests through the apiserver unchanged.
func TestKargoInstance_KargoResourcesRoundTrip(t *testing.T) {
	ctx := context.Background()

	ki := &corev1alpha2.KargoInstance{
		ObjectMeta: newMeta("ki-resources"),
		Spec: corev1alpha2.KargoInstanceResourceSpec{
			ForProvider: corev1alpha2.KargoInstanceParameters{
				Name:  "ki-resources",
				Kargo: corev1alpha2.KargoSpec{Version: "v1.4.0"},
				KargoResources: []runtime.RawExtension{
					mustRawExt(t, map[string]interface{}{
						"apiVersion": "kargo.akuity.io/v1alpha1",
						"kind":       "Project",
						"metadata":   map[string]interface{}{"name": "platform"},
					}),
					mustRawExt(t, map[string]interface{}{
						"apiVersion": "kargo.akuity.io/v1alpha1",
						"kind":       "Warehouse",
						"metadata":   map[string]interface{}{"name": "wh", "namespace": "platform"},
					}),
				},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, ki))
	t.Cleanup(func() { _ = kube.Delete(ctx, ki) })

	got := &corev1alpha2.KargoInstance{}
	require.NoError(t, kube.Get(ctx, client.ObjectKey{Namespace: ki.Namespace, Name: ki.Name}, got))
	require.Len(t, got.Spec.ForProvider.Resources, 2)
}

// TestKargoInstance_RepoCredentialSecretRefs covers the typed
// repo-credential ref field added in 7.B': enum CredType, DNS-1123
// Name + ProjectNamespace, and uniqueness CEL across (projectNamespace,
// name) pairs. Ensures plaintext-free repo-cred declaration is
// admission-friendly.
func TestKargoInstance_RepoCredentialSecretRefs(t *testing.T) {
	ctx := context.Background()

	good := &corev1alpha2.KargoInstance{
		ObjectMeta: newMeta("ki-repocreds-ok"),
		Spec: corev1alpha2.KargoInstanceResourceSpec{
			ForProvider: corev1alpha2.KargoInstanceParameters{
				Name:  "ki-repocreds-ok",
				Kargo: corev1alpha2.KargoSpec{Version: "v1.4.0"},
				KargoRepoCredentialSecretRefs: []corev1alpha2.KargoRepoCredentialSecretRef{{
					Name:             "repo-github",
					ProjectNamespace: "platform",
					CredType:         "git",
					SecretRef:        xpv1.LocalSecretReference{Name: "k8s-secret"},
				}},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, good), "well-formed ref must be accepted")
	t.Cleanup(func() { _ = kube.Delete(ctx, good) })

	badType := &corev1alpha2.KargoInstance{
		ObjectMeta: newMeta("ki-repocreds-badtype"),
		Spec: corev1alpha2.KargoInstanceResourceSpec{
			ForProvider: corev1alpha2.KargoInstanceParameters{
				Name:  "ki-repocreds-badtype",
				Kargo: corev1alpha2.KargoSpec{Version: "v1.4.0"},
				KargoRepoCredentialSecretRefs: []corev1alpha2.KargoRepoCredentialSecretRef{{
					Name:             "repo-github",
					ProjectNamespace: "platform",
					CredType:         "oci", // not in enum
					SecretRef:        xpv1.LocalSecretReference{Name: "k8s-secret"},
				}},
			},
		},
	}
	err := kube.Create(ctx, badType)
	require.Error(t, err, "CredType outside enum must be rejected")
	assert.Contains(t, err.Error(), "credType")

	dup := &corev1alpha2.KargoInstance{
		ObjectMeta: newMeta("ki-repocreds-dup"),
		Spec: corev1alpha2.KargoInstanceResourceSpec{
			ForProvider: corev1alpha2.KargoInstanceParameters{
				Name:  "ki-repocreds-dup",
				Kargo: corev1alpha2.KargoSpec{Version: "v1.4.0"},
				KargoRepoCredentialSecretRefs: []corev1alpha2.KargoRepoCredentialSecretRef{
					{Name: "repo-github", ProjectNamespace: "platform", CredType: "git", SecretRef: xpv1.LocalSecretReference{Name: "a"}},
					{Name: "repo-github", ProjectNamespace: "platform", CredType: "git", SecretRef: xpv1.LocalSecretReference{Name: "b"}},
				},
			},
		},
	}
	err = kube.Create(ctx, dup)
	require.Error(t, err, "duplicate (projectNamespace, name) pair must be rejected by CEL")
	assert.Contains(t, err.Error(), "unique")
}

// TestKargoInstance_SecretInKargoResourcesRejected locks in the 8.B3
// admission-time CEL rule: v1/Secret manifests are refused before they
// can land in etcd. Prior behavior (prove the apiserver admitted them
// and the controller rejected at reconcile) was a plaintext leak at the
// trust boundary — callers with only list/get on the MR could read
// inline credential data. The controller-level rejection in
// splitKargoResources remains as defense-in-depth.
func TestKargoInstance_SecretInKargoResourcesRejected(t *testing.T) {
	ctx := context.Background()

	ki := &corev1alpha2.KargoInstance{
		ObjectMeta: newMeta("ki-inline-secret"),
		Spec: corev1alpha2.KargoInstanceResourceSpec{
			ForProvider: corev1alpha2.KargoInstanceParameters{
				Name:  "ki-inline-secret",
				Kargo: corev1alpha2.KargoSpec{Version: "v1.4.0"},
				KargoResources: []runtime.RawExtension{
					mustRawExt(t, map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata": map[string]interface{}{
							"name": "repo-github",
							"labels": map[string]interface{}{
								"kargo.akuity.io/cred-type": "git",
							},
						},
						"stringData": map[string]interface{}{
							"password": "leaked-if-admitted",
						},
					}),
				},
			},
		},
	}
	err := kube.Create(ctx, ki)
	require.Error(t, err, "apiserver must reject v1/Secret inside kargoResources before it can land in etcd")
	assert.Contains(t, err.Error(), "kargoRepoCredentialSecretRefs")
}

// TestInstance_SecretInArgocdResourcesRejected mirrors the Kargo rule
// for the Instance surface. argocdResources is schemaless, so without
// the parent-level CEL rule a user could leak plaintext credentials
// through the MR spec. Typed *SecretRef fields are the supported path.
func TestInstance_SecretInArgocdResourcesRejected(t *testing.T) {
	ctx := context.Background()

	inst := &corev1alpha2.Instance{
		ObjectMeta: newMeta("inst-inline-secret"),
		Spec: corev1alpha2.InstanceSpec{
			ForProvider: corev1alpha2.InstanceParameters{
				Name:   "inst-inline-secret",
				ArgoCD: &corev1alpha2.ArgoCDSpec{Version: "v2.12.4"},
				ArgocdResources: []runtime.RawExtension{
					mustRawExt(t, map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata":   map[string]interface{}{"name": "repo-github"},
						"stringData": map[string]interface{}{"password": "leaked-if-admitted"},
					}),
				},
			},
		},
	}
	err := kube.Create(ctx, inst)
	require.Error(t, err, "apiserver must reject v1/Secret inside argocdResources")
	assert.Contains(t, err.Error(), "*SecretRef")
}

// TestCluster_MaintenanceModeExpiryRequiresMode exercises the CEL rule
// that guards ClusterData: a maintenance-expiry timestamp without
// maintenanceMode=true is meaningless and is rejected at admission.
func TestCluster_MaintenanceModeExpiryRequiresMode(t *testing.T) {
	ctx := context.Background()

	tt := metav1.NewTime(metav1.Now().Add(3600 * 1_000_000_000))
	trueVal := true

	missingMode := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cl-exp-missing-mode"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID: "inst-abc",
				Name:       "c1",
				Data: corev1alpha2.ClusterData{
					MaintenanceModeExpiry: &tt,
				},
			},
		},
	}
	err := kube.Create(ctx, missingMode)
	require.Error(t, err, "apiserver must reject maintenanceModeExpiry without maintenanceMode=true")
	assert.Contains(t, err.Error(), "maintenanceModeExpiry requires maintenanceMode=true")

	withMode := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cl-exp-with-mode"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID: "inst-abc",
				Name:       "c1",
				Data: corev1alpha2.ClusterData{
					MaintenanceMode:       &trueVal,
					MaintenanceModeExpiry: &tt,
				},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, withMode))
	t.Cleanup(func() { _ = kube.Delete(ctx, withMode) })
}

// TestCluster_AutoscalerConfigSizePairing exercises the twin CEL rules:
// size=custom requires autoscalerConfig, and autoscalerConfig is only
// valid for size in {auto, custom}.
func TestCluster_AutoscalerConfigSizePairing(t *testing.T) {
	ctx := context.Background()

	customSize := corev1alpha2.ClusterSize("custom")
	fixedSize := corev1alpha2.ClusterSize("large")
	autoSize := corev1alpha2.ClusterSize("auto")

	// size=custom without autoscalerConfig is rejected.
	customNoAuto := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cl-custom-no-auto"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID: "inst-abc",
				Name:       "c1",
				Data: corev1alpha2.ClusterData{
					Size: customSize,
				},
			},
		},
	}
	err := kube.Create(ctx, customNoAuto)
	require.Error(t, err, "apiserver must reject size=custom without autoscalerConfig")
	assert.Contains(t, err.Error(), "size=custom requires autoscalerConfig")

	// Fixed size + autoscalerConfig is rejected.
	fixedWithAuto := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cl-fixed-with-auto"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID: "inst-abc",
				Name:       "c1",
				Data: corev1alpha2.ClusterData{
					Size:             fixedSize,
					AutoscalerConfig: &corev1alpha2.AutoScalerConfig{},
				},
			},
		},
	}
	err = kube.Create(ctx, fixedWithAuto)
	require.Error(t, err, "apiserver must reject autoscalerConfig on fixed size")
	assert.Contains(t, err.Error(), "autoscalerConfig only valid for size=auto or size=custom")

	// size=auto + autoscalerConfig is accepted.
	autoGood := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cl-auto-good"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID: "inst-abc",
				Name:       "c1",
				Data: corev1alpha2.ClusterData{
					Size:             autoSize,
					AutoscalerConfig: &corev1alpha2.AutoScalerConfig{},
				},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, autoGood))
	t.Cleanup(func() { _ = kube.Delete(ctx, autoGood) })

	// size=custom + autoscalerConfig is accepted.
	customGood := &corev1alpha2.Cluster{
		ObjectMeta: newMeta("cl-custom-good"),
		Spec: corev1alpha2.ClusterSpec{
			ForProvider: corev1alpha2.ClusterParameters{
				InstanceID: "inst-abc",
				Name:       "c1",
				Data: corev1alpha2.ClusterData{
					Size:             customSize,
					AutoscalerConfig: &corev1alpha2.AutoScalerConfig{},
				},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, customGood))
	t.Cleanup(func() { _ = kube.Delete(ctx, customGood) })
}
