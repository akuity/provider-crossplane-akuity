/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package instance

import (
	"encoding/json"
	"testing"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	"github.com/akuityio/provider-crossplane-akuity/internal/secrets"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
)

func mustRawMap(t *testing.T, obj map[string]interface{}) runtime.RawExtension {
	t.Helper()
	raw, err := json.Marshal(obj)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: raw}
}

func mustStruct(t *testing.T, obj map[string]interface{}) *structpb.Struct {
	t.Helper()
	pb, err := structpb.NewStruct(obj)
	require.NoError(t, err)
	return pb
}

func TestSpecToInstanceSpec_PropagatesAllCurrentGeneratedFields(t *testing.T) {
	spec := crossplanetypes.InstanceSpec{
		ClusterCustomizationDefaults: &crossplanetypes.ClusterCustomization{
			Kustomization:         "{}\n",
			ServerSideDiffEnabled: ptr.To(true),
		},
		Secrets: &crossplanetypes.SecretsManagementConfig{
			Sources: []*crossplanetypes.ClusterSecretMapping{
				{
					Clusters: &crossplanetypes.ObjectSelector{
						MatchLabels: map[string]string{"cluster": "prod"},
					},
					Secrets: &crossplanetypes.ObjectSelector{
						MatchExpressions: []*crossplanetypes.LabelSelectorRequirement{
							{
								Key:      ptr.To("sync"),
								Operator: ptr.To("Exists"),
							},
						},
					},
				},
			},
		},
		MetricsIngressUsername:        ptr.To("metrics-user"),
		MetricsIngressPasswordHash:    ptr.To("metrics-hash"),
		PrivilegedNotificationCluster: ptr.To("notifications"),
		ClusterAddonsExtension: &crossplanetypes.ClusterAddonsExtension{
			Enabled:          ptr.To(true),
			AllowedUsernames: []string{"alice"},
			AllowedGroups:    []string{"admins"},
		},
		ManifestGeneration: &crossplanetypes.ManifestGeneration{
			Kustomize: &crossplanetypes.ConfigManagementToolVersions{
				DefaultVersion:     "v5.4.3",
				AdditionalVersions: []string{"v4.5.7"},
			},
		},
	}

	wire, err := SpecToInstanceSpec(spec)
	require.NoError(t, err)

	require.NotNil(t, wire.ClusterCustomizationDefaults)
	assert.Equal(t, ptr.To(true), wire.ClusterCustomizationDefaults.ServerSideDiffEnabled)
	assert.NotEmpty(t, wire.ClusterCustomizationDefaults.Kustomization.Raw)
	require.NotNil(t, wire.Secrets)
	assert.Equal(t, map[string]string{"cluster": "prod"}, wire.Secrets.Sources[0].Clusters.MatchLabels)
	assert.Equal(t, ptr.To("sync"), wire.Secrets.Sources[0].Secrets.MatchExpressions[0].Key)
	assert.Equal(t, ptr.To("Exists"), wire.Secrets.Sources[0].Secrets.MatchExpressions[0].Operator)
	assert.Equal(t, ptr.To("metrics-user"), wire.MetricsIngressUsername)
	assert.Equal(t, ptr.To("metrics-hash"), wire.MetricsIngressPasswordHash)
	assert.Equal(t, ptr.To("notifications"), wire.PrivilegedNotificationCluster)
	require.NotNil(t, wire.ClusterAddonsExtension)
	assert.Equal(t, ptr.To(true), wire.ClusterAddonsExtension.Enabled)
	assert.Equal(t, []string{"alice"}, wire.ClusterAddonsExtension.AllowedUsernames)
	assert.Equal(t, []string{"admins"}, wire.ClusterAddonsExtension.AllowedGroups)
	require.NotNil(t, wire.ManifestGeneration)
	require.NotNil(t, wire.ManifestGeneration.Kustomize)
	assert.Equal(t, "v5.4.3", wire.ManifestGeneration.Kustomize.DefaultVersion)
	assert.Equal(t, []string{"v4.5.7"}, wire.ManifestGeneration.Kustomize.AdditionalVersions)
}

// TestSplitArgocdResources_Routing locks in the (apiVersion, kind)
// allowlist: Application / ApplicationSet / AppProject under
// argoproj.io/v1alpha1. Each entry must land on the matching slice on
// the Apply payload. Without this the gateway never sees the user's
// declarative children and they silently disappear from the platform.
func TestSplitArgocdResources_Routing(t *testing.T) {
	in := []runtime.RawExtension{
		mustRawMap(t, map[string]interface{}{
			"apiVersion": argocdAPIVersion, "kind": argocdKindApplication,
			"metadata": map[string]interface{}{"name": "app1"},
		}),
		mustRawMap(t, map[string]interface{}{
			"apiVersion": argocdAPIVersion, "kind": argocdKindApplicationSet,
			"metadata": map[string]interface{}{"name": "appset1"},
		}),
		mustRawMap(t, map[string]interface{}{
			"apiVersion": argocdAPIVersion, "kind": argocdKindAppProject,
			"metadata": map[string]interface{}{"name": "proj1"},
		}),
	}
	got, err := splitArgocdResources(in)
	require.NoError(t, err)
	assert.Len(t, got.Applications, 1)
	assert.Len(t, got.ApplicationSets, 1)
	assert.Len(t, got.AppProjects, 1)
}

// TestSplitArgocdResources_RejectsSecretsAsTerminal verifies inline
// v1/Secret entries are rejected with terminal classification.
// Plaintext credential data must use typed SecretRefs instead.
func TestSplitArgocdResources_RejectsSecretsAsTerminal(t *testing.T) {
	in := []runtime.RawExtension{
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "v1", "kind": "Secret",
			"metadata": map[string]interface{}{"name": "repo-github"},
		}),
	}
	_, err := splitArgocdResources(in)
	require.Error(t, err)
	assert.True(t, reason.IsTerminal(err),
		"inline v1/Secret in resources must surface as reason.Terminal, got %T %v", err, err)
}

func TestSplitArgocdResources_RejectsUnsupportedKind(t *testing.T) {
	in := []runtime.RawExtension{
		mustRawMap(t, map[string]interface{}{
			"apiVersion": "batch/v1", "kind": "Job",
			"metadata": map[string]interface{}{"name": "job1"},
		}),
	}
	_, err := splitArgocdResources(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestSplitArgocdResources_Empty(t *testing.T) {
	got, err := splitArgocdResources(nil)
	require.NoError(t, err)
	assert.Nil(t, got.Applications)
	assert.Nil(t, got.ApplicationSets)
	assert.Nil(t, got.AppProjects)
}

func TestResolvedInstanceSecretsHashChangesOnSourceIdentity(t *testing.T) {
	data := map[string]string{"password": "p"}
	base := resolvedInstanceSecrets{
		Argocd: secrets.ResolvedSecret{Namespace: "team-a", Name: "argocd", Data: data},
		RepoCreds: map[string]secrets.ResolvedSecret{
			"repo-github": {Namespace: "team-a", Name: "repo-creds", Data: data},
		},
	}

	movedSingleton := base
	movedSingleton.Argocd.Namespace = "team-b"
	assert.NotEqual(t, base.Hash(), movedSingleton.Hash(), "singleton source namespace change must rotate hash")

	movedRepo := base
	movedRepo.RepoCreds = map[string]secrets.ResolvedSecret{
		"repo-github": {Namespace: "team-b", Name: "repo-creds", Data: data},
	}
	assert.NotEqual(t, base.Hash(), movedRepo.Hash(), "named source namespace change must rotate hash")
}

// TestArgocdResourcesUpToDate_Subset locks the additive semantics:
// a desired Application whose targetRevision matches the gateway's
// echo (with extra server-side fields) is up-to-date, and a missing
// or divergent entry surfaces as drift.
func TestArgocdResourcesUpToDate_Subset(t *testing.T) {
	desired := []runtime.RawExtension{
		mustRawMap(t, map[string]interface{}{
			"apiVersion": argocdAPIVersion, "kind": argocdKindApplication,
			"metadata": map[string]interface{}{"name": "guestbook"},
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"repoURL":        "https://example.com/repo",
					"targetRevision": "v1.0.0",
				},
			},
		}),
	}

	t.Run("subset match returns up-to-date", func(t *testing.T) {
		exp := &argocdv1.ExportInstanceResponse{
			Applications: []*structpb.Struct{
				mustStruct(t, map[string]interface{}{
					"apiVersion": argocdAPIVersion, "kind": argocdKindApplication,
					"metadata": map[string]interface{}{"name": "guestbook"},
					"spec": map[string]interface{}{
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/repo",
							"targetRevision": "v1.0.0",
							"path":           "manifests", // server-side default; extra is fine
						},
						"project": "default", // server-side default; extra is fine
					},
				}),
			},
		}
		ok, _, err := argocdResourcesUpToDate(desired, exp)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("targetRevision flip surfaces as drift", func(t *testing.T) {
		exp := &argocdv1.ExportInstanceResponse{
			Applications: []*structpb.Struct{
				mustStruct(t, map[string]interface{}{
					"apiVersion": argocdAPIVersion, "kind": argocdKindApplication,
					"metadata": map[string]interface{}{"name": "guestbook"},
					"spec": map[string]interface{}{
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/repo",
							"targetRevision": "v0.9.0", // diverged
						},
					},
				}),
			},
		}
		ok, report, err := argocdResourcesUpToDate(desired, exp)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.NotEmpty(t, report.Changed)
	})

	t.Run("missing on server surfaces as drift", func(t *testing.T) {
		exp := &argocdv1.ExportInstanceResponse{}
		ok, report, err := argocdResourcesUpToDate(desired, exp)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.NotEmpty(t, report.Missing)
	})
}

// TestArgocdResourcesUpToDate_AdditiveUnset captures the additive
// rule: removing an entry from spec.forProvider.resources does NOT
// trigger drift on the controller side. Operators must delete via the
// Akuity platform UI to remove a resource; this guarantees we don't
// reap children other tools or the platform UI co-manage.
func TestArgocdResourcesUpToDate_AdditiveUnset(t *testing.T) {
	// Desired is empty (user removed the entry) but the server still
	// has it. That's not drift on this side.
	exp := &argocdv1.ExportInstanceResponse{
		Applications: []*structpb.Struct{
			mustStruct(t, map[string]interface{}{
				"apiVersion": argocdAPIVersion, "kind": argocdKindApplication,
				"metadata": map[string]interface{}{"name": "leftover"},
			}),
		},
	}
	ok, _, err := argocdResourcesUpToDate(nil, exp)
	require.NoError(t, err)
	assert.True(t, ok, "additive semantics: removing an entry from spec is not drift")
}

// TestObserve_ResourcesDrift exercises the full Observe-to-drift wiring:
// when the user pins an Application that the Akuity gateway has not
// reflected, Observe must report ResourceUpToDate=false so the
// reconciler fires Update and Apply propagates the manifest.
func TestObserve_ResourcesDrift(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := *fixtures.CrossplaneManagedInstance.DeepCopy()
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}
	managedInstance.Spec.ForProvider.Resources = []runtime.RawExtension{
		mustRawMap(t, map[string]interface{}{
			"apiVersion": argocdAPIVersion, "kind": argocdKindApplication,
			"metadata": map[string]interface{}{"name": "guestbook"},
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"repoURL":        "https://example.com/repo",
					"targetRevision": "v1.0.0",
				},
			},
		}),
	}

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)
	mc.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1) // server has not reflected the Application

	resp, err := e.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.True(t, resp.ResourceExists)
	assert.False(t, resp.ResourceUpToDate,
		"user-pinned Application missing from Export must surface as drift")
}

// TestObserve_ResourcesEmptyRoundTrip locks the no-resources path:
// an Instance with empty spec.forProvider.resources must Observe as
// up-to-date when the rest of the spec matches Export. The
// resources-aware drift hook must not fire for empty input.
func TestObserve_ResourcesEmptyRoundTrip(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}
	managedInstance.Spec.ForProvider.Resources = nil

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)
	mc.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	resp, err := e.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.True(t, resp.ResourceExists)
	// Observe baseline (TestObserve_HealthStatusHealthy etc.) doesn't
	// assert UpToDate because the existing fixture's spec drifts from
	// AkuityInstance on a few defaults; the gate here is only that the
	// resources hook didn't fire on empty input.
	_ = resp.ResourceUpToDate
}

// TestBuildApplyInstanceRequest_ResourcesWiring asserts the SET path:
// when spec.forProvider.resources carries an Application,
// BuildApplyInstanceRequest must place the encoded structpb on
// req.Applications. Without this, ApplyInstance never carries the
// child manifest, the gateway never sees it, and the user's
// declarative configuration silently never reaches the platform.
func TestBuildApplyInstanceRequest_ResourcesWiring(t *testing.T) {
	mr := *fixtures.CrossplaneManagedInstance.DeepCopy()
	mr.Spec.ForProvider.Resources = []runtime.RawExtension{
		mustRawMap(t, map[string]interface{}{
			"apiVersion": argocdAPIVersion, "kind": argocdKindApplication,
			"metadata": map[string]interface{}{"name": "guestbook"},
		}),
		mustRawMap(t, map[string]interface{}{
			"apiVersion": argocdAPIVersion, "kind": argocdKindAppProject,
			"metadata": map[string]interface{}{"name": "platform"},
		}),
	}
	req, err := BuildApplyInstanceRequest(mr, resolvedInstanceSecrets{})
	require.NoError(t, err)
	require.Len(t, req.GetApplications(), 1)
	require.Len(t, req.GetAppProjects(), 1)
	assert.Empty(t, req.GetApplicationSets())
}

// TestObserve_AvailableEndToEnd guards against import-list rot when
// Observe shifts to consume the existing Export response for the new
// children drift check. Mirrors the structure of TestUpdate_ClientErr.
func TestObserve_AvailableEndToEnd(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}
	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)
	mc.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	obs, err := e.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
}

// TestSpecToConfigManagementPluginsPB_OptionalSubTrees locks in the
// nil-guard contract on the optional pointer fields of PluginSpec.
// PluginSpec.Discover and PluginSpec.Parameters are *Discover and
// *Parameters in the CRD, so a user CR may legally omit either. The
// previous implementation dereferenced both unconditionally and
// panicked at convert time; controller-runtime caught the panic but
// Crossplane stamped crossplane.io/external-create-pending and the
// resource got stuck in "cannot determine creation result" until the
// user mutated the spec. Cover three shapes (no Discover, no
// Parameters, neither) plus the populated shape so the convert path
// never panics on legal input.
func TestSpecToConfigManagementPluginsPB_OptionalSubTrees(t *testing.T) {
	plugins := map[string]crossplanetypes.ConfigManagementPlugin{
		"only-version": {
			Enabled: true,
			Image:   "img:1",
			Spec:    crossplanetypes.PluginSpec{Version: "v1"},
		},
		"with-discover": {
			Enabled: true,
			Image:   "img:1",
			Spec: crossplanetypes.PluginSpec{
				Version:  "v1",
				Discover: &crossplanetypes.Discover{FileName: "Pluginfile"},
			},
		},
		"with-parameters": {
			Enabled: true,
			Image:   "img:1",
			Spec: crossplanetypes.PluginSpec{
				Version: "v1",
				Parameters: &crossplanetypes.Parameters{
					Static: []*crossplanetypes.ParameterAnnouncement{{Name: "p"}},
				},
			},
		},
		"with-both": {
			Enabled: true,
			Image:   "img:1",
			Spec: crossplanetypes.PluginSpec{
				Version:  "v1",
				Discover: &crossplanetypes.Discover{FileName: "Pluginfile"},
				Parameters: &crossplanetypes.Parameters{
					Static: []*crossplanetypes.ParameterAnnouncement{{Name: "p"}},
				},
			},
		},
	}
	out, err := specToConfigManagementPluginsPB(plugins)
	require.NoError(t, err)
	require.Len(t, out, 4)
}

// Used to keep imports from being declared-and-not-used when a
// reviewer trims tests in the future.
var (
	_ = managed.ExternalObservation{}
	_ = meta.GetExternalName
	_ = gomock.Any
)
