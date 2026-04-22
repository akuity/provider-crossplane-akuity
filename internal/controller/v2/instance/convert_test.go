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

package instance

import (
	"context"
	"encoding/json"
	"testing"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
)

func mustRaw(t *testing.T, obj map[string]interface{}) runtime.RawExtension {
	t.Helper()
	b, err := json.Marshal(obj)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: b}
}

func TestBuildApplyRequest_HardcodesCMPPrune(t *testing.T) {
	mg := newInstance()
	req, err := buildApplyRequest(context.Background(), nil, mg, resolvedInstanceSecrets{})
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Len(t, req.GetPruneResourceTypes(), 1)
	assert.Equal(t,
		argocdv1.PruneResourceType_PRUNE_RESOURCE_TYPE_CONFIG_MANAGEMENT_PLUGINS,
		req.GetPruneResourceTypes()[0])
}

func TestBuildApplyRequest_PopulatesResolvedSecrets(t *testing.T) {
	mg := newInstance()
	mg.Spec.ForProvider.ArgoCDSecretRef = &xpv1.LocalSecretReference{Name: "ignored-we-pass-resolved"}

	sec := resolvedInstanceSecrets{
		ArgoCD:         resolvedSecret{Name: "argocd", Data: map[string]string{"admin.password": "hunter2"}},
		Notifications:  resolvedSecret{Name: "notif", Data: map[string]string{"slack.token": "xoxb"}},
		ImageUpdater:   resolvedSecret{Name: "iu", Data: map[string]string{"registry.token": "tok"}},
		ApplicationSet: resolvedSecret{Name: "as", Data: map[string]string{"plugin.token": "pt"}},
		RepoCredentials: []resolvedNamedSecret{
			{Slot: "repo-github", SecretName: "gh", Data: map[string]string{"url": "https://github.com", "password": "ghp_x"}},
			{Slot: "repo-gitlab", SecretName: "gl", Data: map[string]string{"url": "https://gitlab.com", "password": "glpat"}},
		},
		RepoTemplateCreds: []resolvedNamedSecret{
			{Slot: "repo-tmpl", SecretName: "tmpl", Data: map[string]string{"url": "https://*.example.com", "password": "t"}},
		},
	}

	req, err := buildApplyRequest(context.Background(), nil, mg, sec)
	require.NoError(t, err)

	require.NotNil(t, req.GetArgocdSecret())
	require.NotNil(t, req.GetNotificationsSecret())
	require.NotNil(t, req.GetImageUpdaterSecret())
	require.NotNil(t, req.GetApplicationSetSecret())
	require.Len(t, req.GetRepoCredentialSecrets(), 2)
	require.Len(t, req.GetRepoTemplateCredentialSecrets(), 1)

	// Verify each named repo secret carries the argocd secret-type
	// label appropriate to its slot. The struct is a marshalled
	// Secret; asMap() round-trips it through JSON so we can peek.
	for _, s := range req.GetRepoCredentialSecrets() {
		m := s.AsMap()
		md, _ := m["metadata"].(map[string]interface{})
		lbls, _ := md["labels"].(map[string]interface{})
		assert.Equal(t, secretTypeRepository, lbls[secretTypeLabel])
	}
	for _, s := range req.GetRepoTemplateCredentialSecrets() {
		m := s.AsMap()
		md, _ := m["metadata"].(map[string]interface{})
		lbls, _ := md["labels"].(map[string]interface{})
		assert.Equal(t, secretTypeRepoCreds, lbls[secretTypeLabel])
	}
}

func TestBuildApplyRequest_EmptySecretsOmitted(t *testing.T) {
	mg := newInstance()
	req, err := buildApplyRequest(context.Background(), nil, mg, resolvedInstanceSecrets{})
	require.NoError(t, err)
	assert.Nil(t, req.GetArgocdSecret())
	assert.Nil(t, req.GetNotificationsSecret())
	assert.Nil(t, req.GetImageUpdaterSecret())
	assert.Nil(t, req.GetApplicationSetSecret())
	assert.Nil(t, req.GetRepoCredentialSecrets())
	assert.Nil(t, req.GetRepoTemplateCredentialSecrets())
}

func TestResolveInstanceSecrets_ReadsFromNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	adminSec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "admin-creds"},
		Data:       map[string][]byte{"admin.password": []byte("topsecret")},
	}
	ghSec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "github-repo"},
		Data: map[string][]byte{
			"url":      []byte("https://github.com"),
			"password": []byte("ghp_abc"),
		},
	}
	kube := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(adminSec, ghSec).
		Build()

	mg := &v1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inst"},
		Spec: v1alpha2.InstanceSpec{
			ForProvider: v1alpha2.InstanceParameters{
				Name:            "inst",
				ArgoCDSecretRef: &xpv1.LocalSecretReference{Name: "admin-creds"},
				RepoCredentialSecretRefs: []v1alpha2.NamedLocalSecretReference{{
					Name:      "repo-gh",
					SecretRef: xpv1.LocalSecretReference{Name: "github-repo"},
				}},
			},
		},
	}

	resolved, err := resolveInstanceSecrets(context.Background(), kube, mg)
	require.NoError(t, err)

	assert.Equal(t, "admin-creds", resolved.ArgoCD.Name)
	assert.Equal(t, "topsecret", resolved.ArgoCD.Data["admin.password"])
	require.Len(t, resolved.RepoCredentials, 1)
	assert.Equal(t, "repo-gh", resolved.RepoCredentials[0].Slot)
	assert.Equal(t, "github-repo", resolved.RepoCredentials[0].SecretName)
	assert.Equal(t, "ghp_abc", resolved.RepoCredentials[0].Data["password"])

	// Unreferenced fields stay zero.
	assert.Empty(t, resolved.Notifications.Name)
	assert.Nil(t, resolved.Notifications.Data)
	assert.Empty(t, resolved.ApplicationSet.Name)
	assert.Nil(t, resolved.ApplicationSet.Data)
}

func TestResolvedInstanceSecrets_HashChangesOnContentRotation(t *testing.T) {
	a := resolvedInstanceSecrets{ArgoCD: resolvedSecret{Name: "s", Data: map[string]string{"k": "v1"}}}
	b := resolvedInstanceSecrets{ArgoCD: resolvedSecret{Name: "s", Data: map[string]string{"k": "v2"}}}
	assert.NotEqual(t, a.Hash(), b.Hash(), "hash must differ when content differs")
}

// TestResolvedInstanceSecrets_HashChangesOnRefRename covers the
// rename-with-identical-content scenario: if the user redirects the
// secret reference to a different kube Secret that happens to hold
// the same bytes, the controller still needs to register drift so
// the new chain of trust is re-applied. Mixing the ref name into the
// hash input catches this.
func TestResolvedInstanceSecrets_HashChangesOnRefRename(t *testing.T) {
	data := map[string]string{"k": "v"}
	a := resolvedInstanceSecrets{ArgoCD: resolvedSecret{Name: "secret-old", Data: data}}
	b := resolvedInstanceSecrets{ArgoCD: resolvedSecret{Name: "secret-new", Data: data}}
	assert.NotEqual(t, a.Hash(), b.Hash(), "hash must differ when ref rename but content identical")
}

func TestResolvedInstanceSecrets_HashChangesOnNamedSecretRename(t *testing.T) {
	data := map[string]string{"url": "https://github.com", "password": "ghp_x"}
	a := resolvedInstanceSecrets{RepoCredentials: []resolvedNamedSecret{{Slot: "repo-gh", SecretName: "a", Data: data}}}
	b := resolvedInstanceSecrets{RepoCredentials: []resolvedNamedSecret{{Slot: "repo-gh", SecretName: "b", Data: data}}}
	assert.NotEqual(t, a.Hash(), b.Hash(), "hash must include underlying kube Secret name for named creds too")
}

func TestResolvedInstanceSecrets_HashStable(t *testing.T) {
	a := resolvedInstanceSecrets{
		ArgoCD:          resolvedSecret{Name: "s", Data: map[string]string{"k": "v"}},
		RepoCredentials: []resolvedNamedSecret{{Slot: "repo-x", SecretName: "s", Data: map[string]string{"url": "u"}}},
	}
	b := resolvedInstanceSecrets{
		ArgoCD:          resolvedSecret{Name: "s", Data: map[string]string{"k": "v"}},
		RepoCredentials: []resolvedNamedSecret{{Slot: "repo-x", SecretName: "s", Data: map[string]string{"url": "u"}}},
	}
	assert.Equal(t, a.Hash(), b.Hash())
}

func TestResolvedInstanceSecrets_EmptyHashIsEmpty(t *testing.T) {
	assert.Empty(t, resolvedInstanceSecrets{}.Hash())
}

func TestSplitArgocdResources_Grouping(t *testing.T) {
	in := []runtime.RawExtension{
		mustRaw(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]interface{}{"name": "app-a"},
		}),
		mustRaw(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]interface{}{"name": "app-b"},
		}),
		mustRaw(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "ApplicationSet",
			"metadata": map[string]interface{}{"name": "set-a"},
		}),
		mustRaw(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "AppProject",
			"metadata": map[string]interface{}{"name": "proj-a"},
		}),
	}
	apps, sets, projects, err := splitArgocdResources(in)
	require.NoError(t, err)
	assert.Len(t, apps, 2)
	assert.Len(t, sets, 1)
	assert.Len(t, projects, 1)
}

func TestSplitArgocdResources_RejectsUnsupportedKind(t *testing.T) {
	in := []runtime.RawExtension{
		mustRaw(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "PipelineRun",
		}),
	}
	_, _, _, err := splitArgocdResources(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported kind")
}

func TestSplitArgocdResources_RejectsWrongAPIVersion(t *testing.T) {
	in := []runtime.RawExtension{
		mustRaw(t, map[string]interface{}{
			"apiVersion": "tekton.dev/v1", "kind": "Application",
		}),
	}
	_, _, _, err := splitArgocdResources(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported apiVersion")
}

func TestSplitArgocdResources_EmptyEntryFailsLoudly(t *testing.T) {
	_, _, _, err := splitArgocdResources([]runtime.RawExtension{{Raw: nil}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty payload")
}

func TestBuildApplyRequest_WiresArgocdResources(t *testing.T) {
	mg := newInstance()
	mg.Spec.ForProvider.Resources = []runtime.RawExtension{
		mustRaw(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "AppProject",
			"metadata": map[string]interface{}{"name": "proj"},
			"spec": map[string]interface{}{
				"description": "test project",
			},
		}),
	}
	req, err := buildApplyRequest(context.Background(), nil, mg, resolvedInstanceSecrets{})
	require.NoError(t, err)
	require.Len(t, req.GetAppProjects(), 1)
	assert.Nil(t, req.GetApplications())
	assert.Nil(t, req.GetApplicationSets())
}

func TestCarryOverSensitiveRefs(t *testing.T) {
	desired := v1alpha2.InstanceParameters{
		ArgoCDSecretRef: &xpv1.LocalSecretReference{Name: "want"},
		RepoCredentialSecretRefs: []v1alpha2.NamedLocalSecretReference{{
			Name:      "repo-a",
			SecretRef: xpv1.LocalSecretReference{Name: "s-a"},
		}},
	}
	actual := v1alpha2.InstanceParameters{}
	carryOverSensitiveRefs(&desired, &actual)

	require.NotNil(t, actual.ArgoCDSecretRef)
	assert.Equal(t, "want", actual.ArgoCDSecretRef.Name)
	require.Len(t, actual.RepoCredentialSecretRefs, 1)
	assert.Equal(t, "repo-a", actual.RepoCredentialSecretRefs[0].Name)
}
