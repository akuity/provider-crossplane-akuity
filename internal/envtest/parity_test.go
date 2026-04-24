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

// Envtest coverage for the content-shape CEL rules — repo-credential
// name regexes, dex secret mutual exclusion, kargo repo-cred uniqueness,
// cred-type enum — plus a round-trip sanity check that
// preserve-unknown-fields resources slices carry opaque manifests
// through the apiserver unchanged. Reconcile-level rotation / drift
// semantics are covered by fake-kube unit tests in the controller
// packages; envtest locks in the apiserver-enforced pieces that unit
// tests can't observe.

package envtest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

func mustRawExt(t *testing.T, obj map[string]interface{}) runtime.RawExtension {
	t.Helper()
	b, err := json.Marshal(obj)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: b}
}

func strPtr(s string) *string { return &s }

// TestInstance_RepoCredentialNamesMustMatchRegex exercises the CEL rule
// on both spec.forProvider.repoCredentialSecretRefs and
// repoTemplateCredentialSecretRefs: each Name is the gateway-facing
// slot, which must match ^repo-[a-z0-9][a-z0-9-]*$. A mistyped Name is
// a silent footgun at the API layer, so the CRD rejects it at
// admission.
func TestInstance_RepoCredentialNamesMustMatchRegex(t *testing.T) {
	ctx := context.Background()

	badSlot := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-bad-repo-name"},
		Spec: v1alpha1.InstanceSpec{
			ForProvider: v1alpha1.InstanceParameters{
				Name:   "inst-bad-repo-name",
				ArgoCD: minimalArgoCD(),
				RepoCredentialSecretRefs: []v1alpha1.NamedLocalSecretReference{{
					Name:      "github-prod", // missing repo- prefix
					SecretRef: xpv1.LocalSecretReference{Name: "gh-prod"},
				}},
			},
		},
	}
	err := kube.Create(ctx, badSlot)
	require.Error(t, err, "apiserver must reject repoCredentialSecretRefs entry without repo- prefix")
	assert.Contains(t, err.Error(), "repo-")

	badTmpl := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-bad-repo-tmpl"},
		Spec: v1alpha1.InstanceSpec{
			ForProvider: v1alpha1.InstanceParameters{
				Name:   "inst-bad-repo-tmpl",
				ArgoCD: minimalArgoCD(),
				RepoTemplateCredentialSecretRefs: []v1alpha1.NamedLocalSecretReference{{
					Name:      "tmpl",
					SecretRef: xpv1.LocalSecretReference{Name: "tmpl"},
				}},
			},
		},
	}
	err = kube.Create(ctx, badTmpl)
	require.Error(t, err, "apiserver must reject repoTemplateCredentialSecretRefs entry without repo- prefix")
	assert.Contains(t, err.Error(), "repo-")

	good := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-good-repo-name"},
		Spec: v1alpha1.InstanceSpec{
			ForProvider: v1alpha1.InstanceParameters{
				Name:   "inst-good-repo-name",
				ArgoCD: minimalArgoCD(),
				RepoCredentialSecretRefs: []v1alpha1.NamedLocalSecretReference{{
					Name:      "repo-github-prod",
					SecretRef: xpv1.LocalSecretReference{Name: "gh-prod"},
				}},
				RepoTemplateCredentialSecretRefs: []v1alpha1.NamedLocalSecretReference{{
					Name:      "repo-templates",
					SecretRef: xpv1.LocalSecretReference{Name: "tmpl"},
				}},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, good), "repo- prefixed names must satisfy CEL")
	t.Cleanup(func() { _ = kube.Delete(ctx, good) })
}

// TestInstance_RepoCredentialNameRegexRejectsBypasses covers bypasses a
// bare startsWith("repo-") rule would accept: whitespace, uppercase,
// prefix alone, leading dash, unicode. The regex
// ^repo-[a-z0-9][a-z0-9-]*$ must reject each at admission so malformed
// gateway slot names never hit Apply.
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
		tc, i := tc, i
		t.Run(tc.desc, func(t *testing.T) {
			inst := &v1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("inst-regex-%d", i)},
				Spec: v1alpha1.InstanceSpec{
					ForProvider: v1alpha1.InstanceParameters{
						Name:   fmt.Sprintf("inst-regex-%d", i),
						ArgoCD: minimalArgoCD(),
						RepoCredentialSecretRefs: []v1alpha1.NamedLocalSecretReference{{
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
// preserve-unknown-fields schema on spec.forProvider.resources carries
// opaque manifests through the apiserver unchanged. Shape / kind
// validation lives in the unit tests for splitArgocdResources — this
// test asserts persistence only.
func TestInstance_ArgocdResourcesRoundTrip(t *testing.T) {
	ctx := context.Background()

	inst := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-argocd-resources"},
		Spec: v1alpha1.InstanceSpec{
			ForProvider: v1alpha1.InstanceParameters{
				Name:   "inst-argocd-resources",
				ArgoCD: minimalArgoCD(),
				Resources: []runtime.RawExtension{
					mustRawExt(t, map[string]interface{}{
						"apiVersion": "argoproj.io/v1alpha1",
						"kind":       "AppProject",
						"metadata":   map[string]interface{}{"name": "team-a"},
						"spec":       map[string]interface{}{"description": "team a"},
					}),
				},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, inst))
	t.Cleanup(func() { _ = kube.Delete(ctx, inst) })

	got := &v1alpha1.Instance{}
	require.NoError(t, kube.Get(ctx, client.ObjectKey{Name: inst.Name}, got))
	require.Len(t, got.Spec.ForProvider.Resources, 1)

	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(got.Spec.ForProvider.Resources[0].Raw, &obj))
	assert.Equal(t, "argoproj.io/v1alpha1", obj["apiVersion"])
	assert.Equal(t, "AppProject", obj["kind"])
}

// TestKargoInstance_DexMutualExclusion exercises the CEL rule that
// forbids setting both spec.forProvider.kargo.oidcConfig.dexConfigSecret
// (inline map) and dexConfigSecretRef simultaneously. The rule uses
// size(...)==0 as the vacuous-inline escape hatch so an empty inline
// map doesn't block ref-only callers; this test pins the behaviour.
func TestKargoInstance_DexMutualExclusion(t *testing.T) {
	ctx := context.Background()

	both := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki-dex-both"},
		Spec: v1alpha1.KargoInstanceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Name: "ki-dex-both",
				Kargo: crossplanetypes.KargoSpec{
					Description: "envtest",
					Version:     "v1.4.0",
					OidcConfig: &crossplanetypes.KargoOidcConfig{
						DexConfigSecret: map[string]crossplanetypes.Value{
							"client-secret": {Value: strPtr("inline-secret")},
						},
						DexConfigSecretRef: &corev1.LocalObjectReference{Name: "dex-creds"},
					},
				},
			},
		},
	}
	err := kube.Create(ctx, both)
	require.Error(t, err, "apiserver must reject both dexConfigSecret and dexConfigSecretRef")
	assert.Contains(t, err.Error(), "set either kargo.oidcConfig.dexConfigSecretRef or kargo.oidcConfig.dexConfigSecret")

	onlyRef := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki-dex-ref-only"},
		Spec: v1alpha1.KargoInstanceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Name: "ki-dex-ref-only",
				Kargo: crossplanetypes.KargoSpec{
					Description: "envtest",
					Version:     "v1.4.0",
					OidcConfig: &crossplanetypes.KargoOidcConfig{
						DexConfigSecretRef: &corev1.LocalObjectReference{Name: "dex-creds"},
					},
				},
			},
		},
	}
	require.NoError(t, kube.Create(ctx, onlyRef), "dexConfigSecretRef alone must satisfy CEL")
	t.Cleanup(func() { _ = kube.Delete(ctx, onlyRef) })

	onlyInline := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki-dex-inline-only"},
		Spec: v1alpha1.KargoInstanceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Name: "ki-dex-inline-only",
				Kargo: crossplanetypes.KargoSpec{
					Description: "envtest",
					Version:     "v1.4.0",
					OidcConfig: &crossplanetypes.KargoOidcConfig{
						DexConfigSecret: map[string]crossplanetypes.Value{
							"client-secret": {Value: strPtr("inline-secret")},
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

	ki := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki-resources"},
		Spec: v1alpha1.KargoInstanceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Name:  "ki-resources",
				Kargo: minimalKargoSpec(),
				Resources: []runtime.RawExtension{
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

	got := &v1alpha1.KargoInstance{}
	require.NoError(t, kube.Get(ctx, client.ObjectKey{Name: ki.Name}, got))
	require.Len(t, got.Spec.ForProvider.Resources, 2)
}

// TestKargoInstance_RepoCredentialSecretRefs covers the typed
// repo-credential ref field: CredType enum, DNS-1123 Name +
// ProjectNamespace, and uniqueness CEL across (projectNamespace, name)
// pairs. Keeps plaintext-free repo-cred declaration admission-friendly.
func TestKargoInstance_RepoCredentialSecretRefs(t *testing.T) {
	ctx := context.Background()

	good := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki-repocreds-ok"},
		Spec: v1alpha1.KargoInstanceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Name:  "ki-repocreds-ok",
				Kargo: minimalKargoSpec(),
				KargoRepoCredentialSecretRefs: []v1alpha1.KargoRepoCredentialSecretRef{{
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

	badType := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki-repocreds-badtype"},
		Spec: v1alpha1.KargoInstanceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Name:  "ki-repocreds-badtype",
				Kargo: minimalKargoSpec(),
				KargoRepoCredentialSecretRefs: []v1alpha1.KargoRepoCredentialSecretRef{{
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

	dup := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki-repocreds-dup"},
		Spec: v1alpha1.KargoInstanceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Name:  "ki-repocreds-dup",
				Kargo: minimalKargoSpec(),
				KargoRepoCredentialSecretRefs: []v1alpha1.KargoRepoCredentialSecretRef{
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
