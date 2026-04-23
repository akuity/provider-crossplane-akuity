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

package kargoinstance

import (
	"context"
	"encoding/json"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

func mustRawMap(t *testing.T, obj map[string]interface{}) runtime.RawExtension {
	t.Helper()
	b, err := json.Marshal(obj)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: b}
}

func TestResolveKargoSecret_NilRefReturnsNil(t *testing.T) {
	mg := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns"},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	kube := fake.NewClientBuilder().WithScheme(scheme).Build()
	got, err := resolveKargoSecrets(context.Background(), kube, mg)
	require.NoError(t, err)
	assert.Empty(t, got.Kargo.Name)
	assert.Nil(t, got.Kargo.Data)
	assert.Empty(t, got.DexConfig.Name)
	assert.Nil(t, got.DexConfig.Data)
	assert.Empty(t, got.Hash())
}

func TestResolveKargoSecret_ReadsSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "kargo-admin"},
		Data:       map[string][]byte{"adminPassword": []byte("p@ss")},
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()

	mg := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
		Spec: v1alpha1.KargoInstanceResourceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				KargoSecretRef: &xpv1.LocalSecretReference{Name: "kargo-admin"},
			},
		},
	}
	got, err := resolveKargoSecrets(context.Background(), kube, mg)
	require.NoError(t, err)
	assert.Equal(t, "kargo-admin", got.Kargo.Name)
	assert.Equal(t, "p@ss", got.Kargo.Data["adminPassword"])
}

func TestKubeSecretToPB_EmptyOmitted(t *testing.T) {
	pb, err := kubeSecretToPB(nil)
	require.NoError(t, err)
	assert.Nil(t, pb)

	pb, err = kubeSecretToPB(map[string]string{})
	require.NoError(t, err)
	assert.Nil(t, pb)
}

func TestKubeSecretToPB_Populated(t *testing.T) {
	pb, err := kubeSecretToPB(map[string]string{"k": "v"})
	require.NoError(t, err)
	require.NotNil(t, pb)
	m := pb.AsMap()
	md, _ := m["metadata"].(map[string]interface{})
	assert.Equal(t, "kargo-secret", md["name"])
}

func TestBuildKargoConfigMapPB_Populated(t *testing.T) {
	pb, err := buildKargoConfigMapPB(map[string]string{"a": "b"}, false)
	require.NoError(t, err)
	require.NotNil(t, pb)
	m := pb.AsMap()
	md, _ := m["metadata"].(map[string]interface{})
	assert.Equal(t, kargoCMKey, md["name"])
	data, _ := m["data"].(map[string]interface{})
	assert.Equal(t, "b", data["a"])
}

// TestBuildKargoConfigMapPB_TombstoneSendsEmptyStruct covers B1:
// an empty desired + tombstoneOnEmpty=true must serialise an empty
// ConfigMap (not nil), otherwise the gateway has no way to
// understand "clear previously-applied keys" on Apply.
func TestBuildKargoConfigMapPB_TombstoneSendsEmptyStruct(t *testing.T) {
	pb, err := buildKargoConfigMapPB(nil, true)
	require.NoError(t, err)
	require.NotNil(t, pb, "tombstone must send an empty ConfigMap, not nil")
	m := pb.AsMap()
	md, _ := m["metadata"].(map[string]interface{})
	assert.Equal(t, kargoCMKey, md["name"])
	data, _ := m["data"].(map[string]interface{})
	assert.Empty(t, data, "tombstone data map must be empty")

	// Empty + not tombstone still returns nil (field never applied).
	pb, err = buildKargoConfigMapPB(nil, false)
	require.NoError(t, err)
	assert.Nil(t, pb, "never-applied empty map must yield nil so the gateway leaves the CM alone")
}

// TestHashKargoConfigMap_EmptyIsEmpty makes the never-applied signal
// unambiguous: empty input = empty digest, so comparing against the
// stored hash reliably distinguishes "never applied" from "applied
// and cleared" (stored hash non-empty → tombstone needed).
func TestHashKargoConfigMap_EmptyIsEmpty(t *testing.T) {
	assert.Empty(t, hashKargoConfigMap(nil))
	assert.Empty(t, hashKargoConfigMap(map[string]string{}))
	assert.NotEmpty(t, hashKargoConfigMap(map[string]string{"k": "v"}))
}

// TestHashKargoConfigMap_DetectsKeyRemoval is the core regression
// guard for B1: removing a key from the desired map must produce a
// different digest so Observe can trigger re-Apply.
func TestHashKargoConfigMap_DetectsKeyRemoval(t *testing.T) {
	a := hashKargoConfigMap(map[string]string{"foo": "bar", "baz": "qux"})
	b := hashKargoConfigMap(map[string]string{"foo": "bar"})
	assert.NotEqual(t, a, b, "removing a key must rotate the digest")
}

func TestResolveKargoSecrets_ResolvesDex(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	dex := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "dex-creds"},
		Data: map[string][]byte{
			"github-client-secret": []byte("gh-xyz"),
			"google-client-secret": []byte("goog-abc"),
		},
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dex).Build()

	mg := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
		Spec: v1alpha1.KargoInstanceResourceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Kargo: crossplanetypes.KargoSpec{
					OidcConfig: &crossplanetypes.KargoOidcConfig{
						DexConfigSecretRef: &corev1.LocalObjectReference{Name: "dex-creds"},
					},
				},
			},
		},
	}
	got, err := resolveKargoSecrets(context.Background(), kube, mg)
	require.NoError(t, err)
	assert.Empty(t, got.Kargo.Name)
	assert.Nil(t, got.Kargo.Data)
	assert.Equal(t, "dex-creds", got.DexConfig.Name)
	assert.Equal(t, "gh-xyz", got.DexConfig.Data["github-client-secret"])
	assert.NotEmpty(t, got.Hash())
}

// TestResolvedKargoSecrets_HashChangesOnRefRename ensures a rename of
// the backing kube Secret (same content) rotates the digest too. See
// the Instance-side equivalent for the rationale.
func TestResolvedKargoSecrets_HashChangesOnRefRename(t *testing.T) {
	data := map[string]string{"k": "v"}
	a := resolvedKargoSecrets{Kargo: kargoResolvedSecret{Name: "old", Data: data}}
	b := resolvedKargoSecrets{Kargo: kargoResolvedSecret{Name: "new", Data: data}}
	assert.NotEqual(t, a.Hash(), b.Hash())
}

func TestSpecToPB_InjectsResolvedDex(t *testing.T) {
	in := v1alpha1.KargoInstanceParameters{
		Name: "my-kargo",
		Kargo: crossplanetypes.KargoSpec{
			Version: "v1.4.0",
			OidcConfig: &crossplanetypes.KargoOidcConfig{
				ClientID:           "app",
				DexConfigSecretRef: &corev1.LocalObjectReference{Name: "dex-creds"},
			},
		},
	}
	resolved := map[string]string{
		"github-client-secret": "gh-xyz",
	}
	pb, err := specToPB(in, resolved)
	require.NoError(t, err)
	require.NotNil(t, pb)

	m := pb.AsMap()
	spec, _ := m["spec"].(map[string]interface{})
	oidc, _ := spec["oidcConfig"].(map[string]interface{})
	dex, _ := oidc["dexConfigSecret"].(map[string]interface{})
	require.NotNil(t, dex, "dexConfigSecret should be populated on the wire payload")
	ghEntry, _ := dex["github-client-secret"].(map[string]interface{})
	require.NotNil(t, ghEntry, "each entry must be wrapped as {value: ...}")
	assert.Equal(t, "gh-xyz", ghEntry["value"])
}

func TestSpecToPB_NoResolvedDexLeavesSpecAlone(t *testing.T) {
	in := v1alpha1.KargoInstanceParameters{
		Name: "my-kargo",
		Kargo: crossplanetypes.KargoSpec{
			Version:    "v1.4.0",
			OidcConfig: &crossplanetypes.KargoOidcConfig{ClientID: "app"},
		},
	}
	pb, err := specToPB(in, nil)
	require.NoError(t, err)
	require.NotNil(t, pb)

	m := pb.AsMap()
	spec, _ := m["spec"].(map[string]interface{})
	oidc, _ := spec["oidcConfig"].(map[string]interface{})
	// The wire field lacks omitempty so the key always appears, but
	// when neither ref nor inline map is set the value is nil/empty.
	dex := oidc["dexConfigSecret"]
	if dex != nil {
		asMap, ok := dex.(map[string]interface{})
		require.True(t, ok, "dexConfigSecret should be an object when present")
		assert.Empty(t, asMap, "no ref + no inline => empty map on wire")
	}
}

func TestSplitKargoResources_Routing(t *testing.T) {
	in := []runtime.RawExtension{
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "kargo.akuity.io/v1alpha1", "kind": "Project",
			"metadata": map[string]interface{}{"name": "app"},
		}),
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "kargo.akuity.io/v1alpha1", "kind": "Warehouse",
			"metadata": map[string]interface{}{"name": "wh"},
		}),
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "kargo.akuity.io/v1alpha1", "kind": "Stage",
			"metadata": map[string]interface{}{"name": "dev"},
		}),
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "kargo.akuity.io/v1alpha1", "kind": "AnalysisTemplate",
			"metadata": map[string]interface{}{"name": "canary"},
		}),
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "kargo.akuity.io/v1alpha1", "kind": "PromotionTask",
			"metadata": map[string]interface{}{"name": "prom"},
		}),
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "kargo.akuity.io/v1alpha1", "kind": "ClusterPromotionTask",
			"metadata": map[string]interface{}{"name": "cprom"},
		}),
	}
	got, err := splitKargoResources(in)
	require.NoError(t, err)
	assert.Len(t, got.Projects, 1)
	assert.Len(t, got.Warehouses, 1)
	assert.Len(t, got.Stages, 1)
	assert.Len(t, got.AnalysisTemplates, 1)
	assert.Len(t, got.PromotionTasks, 1)
	assert.Len(t, got.ClusterPromotionTasks, 1)
}

// TestSplitKargoResources_RejectsSecrets locks in the post-7.B' rule
// that v1/Secret manifests are no longer accepted inside
// spec.forProvider.kargoResources. Repo-credential Secrets must go
// through spec.forProvider.kargoRepoCredentialSecretRefs so plaintext
// stays out of the MR spec and rotation participates in SecretHash
// drift rather than the export-based child index.
func TestSplitKargoResources_RejectsSecrets(t *testing.T) {
	in := []runtime.RawExtension{
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "v1", "kind": "Secret",
			"metadata": map[string]interface{}{
				"name":   "repo-github",
				"labels": map[string]interface{}{kargoCredTypeLabel: "git"},
			},
		}),
	}
	_, err := splitKargoResources(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kargoRepoCredentialSecretRefs")
}

func TestSplitKargoResources_RejectsUnsupportedKind(t *testing.T) {
	in := []runtime.RawExtension{
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "batch/v1", "kind": "Job",
		}),
	}
	_, err := splitKargoResources(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestSplitKargoResources_Empty(t *testing.T) {
	got, err := splitKargoResources(nil)
	require.NoError(t, err)
	assert.Nil(t, got.Projects)
}

func TestSecretHashStatusRoundTrip(t *testing.T) {
	mg := &v1alpha1.KargoInstance{}
	assert.Empty(t, getSecretHash(mg))

	setSecretHash(mg, "abc123")
	assert.Equal(t, "abc123", getSecretHash(mg))
	assert.Equal(t, "abc123", mg.Status.AtProvider.SecretHash,
		"hash lives on status, not annotations, so managed.Reconciler persists it")

	setSecretHash(mg, "")
	assert.Empty(t, getSecretHash(mg))
}

// TestResolveKargoSecrets_ResolvesRepoCredentials exercises the 7.B'
// typed ref path: the controller reads each ref's backing Secret, keeps
// slot / project ns / cred type in the resolved struct, and participates
// in the digest so rotation fires Observe drift even though the gateway
// Export response has no repo_credentials field.
func TestResolveKargoSecrets_ResolvesRepoCredentials(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "repo-github-k8s"},
		Data: map[string][]byte{
			"repoURL":  []byte("https://github.com/acme/platform.git"),
			"username": []byte("bot"),
			"password": []byte("ghp-xyz"),
		},
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()
	mg := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
		Spec: v1alpha1.KargoInstanceResourceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				KargoRepoCredentialSecretRefs: []v1alpha1.KargoRepoCredentialSecretRef{{
					Name:             "repo-github",
					ProjectNamespace: "platform",
					CredType:         "git",
					SecretRef:        xpv1.LocalSecretReference{Name: "repo-github-k8s"},
				}},
			},
		},
	}
	got, err := resolveKargoSecrets(context.Background(), kube, mg)
	require.NoError(t, err)
	require.Len(t, got.RepoCredentials, 1)
	rc := got.RepoCredentials[0]
	assert.Equal(t, "repo-github", rc.Slot)
	assert.Equal(t, "platform", rc.ProjectNamespace)
	assert.Equal(t, "git", rc.CredType)
	assert.Equal(t, "repo-github-k8s", rc.SecretName)
	assert.Equal(t, "ghp-xyz", rc.Data["password"])
	assert.NotEmpty(t, got.Hash(), "repo-cred resolution must contribute to the digest")
}

// TestResolveKargoSecrets_RepoCredsRejectsDuplicateSlot guards against
// a CEL bypass (e.g. admission race) for (projectNamespace, name)
// uniqueness: the controller runs the check again at reconcile time.
func TestResolveKargoSecrets_RepoCredsRejectsDuplicateSlot(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "s"},
		Data:       map[string][]byte{"password": []byte("x")},
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sec).Build()
	mg := &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
		Spec: v1alpha1.KargoInstanceResourceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				KargoRepoCredentialSecretRefs: []v1alpha1.KargoRepoCredentialSecretRef{
					{Name: "repo-github", ProjectNamespace: "platform", CredType: "git", SecretRef: xpv1.LocalSecretReference{Name: "s"}},
					{Name: "repo-github", ProjectNamespace: "platform", CredType: "git", SecretRef: xpv1.LocalSecretReference{Name: "s"}},
				},
			},
		},
	}
	_, err := resolveKargoSecrets(context.Background(), kube, mg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

// TestKargoRepoCredsToPB_ShapeAndLabels confirms the synthesised Secret
// carries the Kargo cred-type label, lands in the declared project
// namespace, and omits entries with empty data.
func TestKargoRepoCredsToPB_ShapeAndLabels(t *testing.T) {
	pbs, err := kargoRepoCredsToPB([]kargoResolvedRepoCred{
		{Slot: "repo-github", ProjectNamespace: "platform", CredType: "git", SecretName: "s", Data: map[string]string{"password": "p"}},
		{Slot: "repo-empty", ProjectNamespace: "platform", CredType: "helm", SecretName: "e", Data: map[string]string{}},
	})
	require.NoError(t, err)
	require.Len(t, pbs, 1, "entries with empty data are dropped")

	m := pbs[0].AsMap()
	md, _ := m["metadata"].(map[string]interface{})
	require.NotNil(t, md)
	assert.Equal(t, "repo-github", md["name"])
	assert.Equal(t, "platform", md["namespace"])
	labels, _ := md["labels"].(map[string]interface{})
	require.NotNil(t, labels)
	assert.Equal(t, "git", labels[kargoCredTypeLabel])
}

// TestKargoRepoCreds_HashChangesOnIdentityRotation covers every rotation
// axis that should fire drift: slot rename, cred-type change, backing
// Secret name change, and payload change.
func TestKargoRepoCreds_HashChangesOnIdentityRotation(t *testing.T) {
	base := kargoResolvedRepoCred{Slot: "repo-github", ProjectNamespace: "platform", CredType: "git", SecretName: "s1", Data: map[string]string{"k": "v"}}
	h := func(c kargoResolvedRepoCred) string {
		return resolvedKargoSecrets{RepoCredentials: []kargoResolvedRepoCred{c}}.Hash()
	}
	original := h(base)

	rename := base
	rename.Slot = "repo-github-2"
	assert.NotEqual(t, original, h(rename), "slot rename must rotate hash")

	credType := base
	credType.CredType = "helm"
	assert.NotEqual(t, original, h(credType), "credType change must rotate hash")

	secRename := base
	secRename.SecretName = "s2"
	assert.NotEqual(t, original, h(secRename), "backing Secret rename must rotate hash")

	payload := base
	payload.Data = map[string]string{"k": "v2"}
	assert.NotEqual(t, original, h(payload), "payload change must rotate hash")
}
