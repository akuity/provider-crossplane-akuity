package children

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime"
)

func rawOf(t *testing.T, obj map[string]interface{}) runtime.RawExtension {
	t.Helper()
	b, err := json.Marshal(obj)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: b}
}

func structOf(t *testing.T, obj map[string]interface{}) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(obj)
	require.NoError(t, err)
	return s
}

func TestIndex_EmptyReturnsNil(t *testing.T) {
	got, err := Index(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestIndex_RejectsMissingName(t *testing.T) {
	_, err := Index([]runtime.RawExtension{
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{},
		}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.name")
}

func TestIndex_RejectsDuplicates(t *testing.T) {
	_, err := Index([]runtime.RawExtension{
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "dup"},
		}),
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "dup"},
		}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

// TestIndexStructs_StripsServerFields confirms that status and
// server-managed metadata (resourceVersion, uid, ...) are stripped
// before the decoded object is handed to the comparator, so those
// fields can't masquerade as drift.
func TestIndexStructs_StripsServerFields(t *testing.T) {
	observed, err := IndexStructs([]*structpb.Struct{
		structOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":            "app",
				"resourceVersion": "12345",
				"uid":             "abcd",
			},
			"spec":   map[string]interface{}{"project": "team-a"},
			"status": map[string]interface{}{"sync": "Synced"},
		}),
	})
	require.NoError(t, err)

	id := Identity{APIVersion: "argoproj.io/v1alpha1", Kind: "Application", Name: "app"}
	obj := observed[id]
	require.NotNil(t, obj)
	md, _ := obj["metadata"].(map[string]interface{})
	require.NotNil(t, md)
	assert.NotContains(t, md, "resourceVersion")
	assert.NotContains(t, md, "uid")
	assert.NotContains(t, obj, "status")
}

// TestCompare_SubsetDefaultsDoNotDrift locks in the 7.D fix: a
// server-defaulted field (spec.project="default") that is absent
// from the user's manifest must not fire drift. Without subset
// semantics, this produced a permanent reapply loop on every
// reconcile for ArgoCD Applications.
func TestCompare_SubsetDefaultsDoNotDrift(t *testing.T) {
	desired, err := Index([]runtime.RawExtension{
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app"},
			"spec": map[string]interface{}{
				"source": map[string]interface{}{"repoURL": "https://github.com/a", "path": "x"},
			},
		}),
	})
	require.NoError(t, err)

	observed, err := IndexStructs([]*structpb.Struct{
		structOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app"},
			"spec": map[string]interface{}{
				"source":     map[string]interface{}{"repoURL": "https://github.com/a", "path": "x"},
				"project":    "default",
				"syncPolicy": map[string]interface{}{"automated": map[string]interface{}{"prune": false}},
			},
		}),
	})
	require.NoError(t, err)

	report := Compare(desired, observed)
	assert.True(t, report.Empty(), "server-defaulted extras must not fire drift")
}

func TestCompare_AdditiveSemantics(t *testing.T) {
	desired, err := Index([]runtime.RawExtension{
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]interface{}{"name": "a"},
			"spec":     map[string]interface{}{"replicas": float64(3)},
		}),
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]interface{}{"name": "b"},
			"spec":     map[string]interface{}{"replicas": float64(1)},
		}),
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]interface{}{"name": "c"},
			"spec":     map[string]interface{}{"replicas": float64(1)},
		}),
	})
	require.NoError(t, err)

	observedExact := func(t *testing.T) map[Identity]map[string]interface{} {
		o, err := IndexStructs([]*structpb.Struct{
			structOf(t, map[string]interface{}{
				"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
				"metadata": map[string]interface{}{"name": "a"},
				"spec":     map[string]interface{}{"replicas": float64(3)},
			}),
			structOf(t, map[string]interface{}{
				"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
				"metadata": map[string]interface{}{"name": "b"},
				"spec":     map[string]interface{}{"replicas": float64(1)},
			}),
			structOf(t, map[string]interface{}{
				"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
				"metadata": map[string]interface{}{"name": "c"},
				"spec":     map[string]interface{}{"replicas": float64(1)},
			}),
		})
		require.NoError(t, err)
		return o
	}

	t.Run("all present and equal: no drift", func(t *testing.T) {
		assert.True(t, Compare(desired, observedExact(t)).Empty())
	})

	t.Run("extra observed entries ignored", func(t *testing.T) {
		o := observedExact(t)
		extra, err := IndexStructs([]*structpb.Struct{
			structOf(t, map[string]interface{}{
				"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
				"metadata": map[string]interface{}{"name": "extra"},
				"spec":     map[string]interface{}{"replicas": float64(99)},
			}),
		})
		require.NoError(t, err)
		for k, v := range extra {
			o[k] = v
		}
		assert.True(t, Compare(desired, o).Empty(), "extra server-side children must not trigger drift")
	})

	t.Run("missing desired fires drift", func(t *testing.T) {
		o := observedExact(t)
		delete(o, Identity{APIVersion: "argoproj.io/v1alpha1", Kind: "Application", Name: "c"})
		report := Compare(desired, o)
		assert.Len(t, report.Missing, 1)
		assert.Equal(t, "c", report.Missing[0].Name)
		assert.Empty(t, report.Changed)
	})

	t.Run("divergent desired path fires drift", func(t *testing.T) {
		o, err := IndexStructs([]*structpb.Struct{
			structOf(t, map[string]interface{}{
				"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
				"metadata": map[string]interface{}{"name": "a"},
				"spec":     map[string]interface{}{"replicas": float64(5)}, // diverges from desired=3
			}),
			structOf(t, map[string]interface{}{
				"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
				"metadata": map[string]interface{}{"name": "b"},
				"spec":     map[string]interface{}{"replicas": float64(1)},
			}),
			structOf(t, map[string]interface{}{
				"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
				"metadata": map[string]interface{}{"name": "c"},
				"spec":     map[string]interface{}{"replicas": float64(1)},
			}),
		})
		require.NoError(t, err)
		report := Compare(desired, o)
		assert.Len(t, report.Changed, 1)
		assert.Equal(t, "a", report.Changed[0].Name)
		assert.Empty(t, report.Missing)
	})
}

func TestCompare_EmptyDesired(t *testing.T) {
	observed, err := IndexStructs([]*structpb.Struct{
		structOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
			"metadata": map[string]interface{}{"name": "a"},
		}),
	})
	require.NoError(t, err)
	assert.True(t, Compare(nil, observed).Empty())
}

// TestMatchesSubset_ArrayExactMatch keeps array semantics strict:
// length and position must agree, since Kubernetes containers, env
// entries, etc. are order-sensitive and an operator shuffling them
// deserves a drift signal.
func TestMatchesSubset_ArrayExactMatch(t *testing.T) {
	assert.True(t, matchesSubset(
		[]interface{}{"a", "b"},
		[]interface{}{"a", "b"},
	))
	assert.False(t, matchesSubset(
		[]interface{}{"a", "b"},
		[]interface{}{"b", "a"}, // reordered
	))
	assert.False(t, matchesSubset(
		[]interface{}{"a"},
		[]interface{}{"a", "b"}, // extra element
	))
}

// TestCompare_FinalizerRemovedServerSideFiresDrift covers the 8.B2
// fix: lifecycle fields that live on metadata (finalizers,
// ownerReferences) must participate in drift detection. If the user
// declared a finalizer and the server dropped it, the next reconcile
// must re-Apply.
func TestCompare_FinalizerRemovedServerSideFiresDrift(t *testing.T) {
	desired, err := Index([]runtime.RawExtension{
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":       "app",
				"finalizers": []interface{}{"resources-finalizer.argocd.argoproj.io"},
			},
			"spec": map[string]interface{}{"project": "team"},
		}),
	})
	require.NoError(t, err)

	observed, err := IndexStructs([]*structpb.Struct{
		structOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app"}, // finalizer dropped
			"spec":       map[string]interface{}{"project": "team"},
		}),
	})
	require.NoError(t, err)

	report := Compare(desired, observed)
	assert.Len(t, report.Changed, 1, "missing finalizer must fire drift")
}

// TestCompare_OwnerReferenceChangedServerSideFiresDrift mirrors the
// finalizer test for ownerReferences. Both fields are user-owned
// lifecycle signals and must not be silently masked.
func TestCompare_OwnerReferenceChangedServerSideFiresDrift(t *testing.T) {
	desired, err := Index([]runtime.RawExtension{
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name": "app",
				"ownerReferences": []interface{}{
					map[string]interface{}{"apiVersion": "v1", "kind": "Foo", "name": "bar", "uid": "u1"},
				},
			},
		}),
	})
	require.NoError(t, err)

	observed, err := IndexStructs([]*structpb.Struct{
		structOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app"},
		}),
	})
	require.NoError(t, err)

	report := Compare(desired, observed)
	assert.Len(t, report.Changed, 1, "missing ownerReference must fire drift")
}

// TestCompare_NamespaceDefaulting covers H2: a desired manifest with
// metadata.namespace omitted must still match an observed resource of
// the same apiVersion+kind+name that landed in a concrete namespace
// (e.g. "default" or a project namespace set by the gateway).
func TestCompare_NamespaceDefaulting(t *testing.T) {
	desired, err := Index([]runtime.RawExtension{
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app"},
			"spec":       map[string]interface{}{"project": "team-a"},
		}),
	})
	require.NoError(t, err)

	observed, err := IndexStructs([]*structpb.Struct{
		structOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app", "namespace": "default"},
			"spec":       map[string]interface{}{"project": "team-a"},
		}),
	})
	require.NoError(t, err)

	report := Compare(desired, observed)
	assert.True(t, report.Empty(), "empty desired namespace must match the single observed entry with a defaulted namespace")
}

// TestCompare_NamespaceAmbiguityIsSafe guards against the obvious
// mismatch: if two observed resources share (apiVersion, kind, name)
// in different namespaces, a namespace-less desired manifest is
// ambiguous and must not silently bind to either. Behaviour: report
// as missing so the operator sees the failure mode, not a phantom
// drift on the wrong twin.
func TestCompare_NamespaceAmbiguityIsSafe(t *testing.T) {
	desired, err := Index([]runtime.RawExtension{
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app"},
		}),
	})
	require.NoError(t, err)

	observed, err := IndexStructs([]*structpb.Struct{
		structOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app", "namespace": "default"},
		}),
		structOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app", "namespace": "staging"},
		}),
	})
	require.NoError(t, err)

	report := Compare(desired, observed)
	assert.Len(t, report.Missing, 1, "two observed candidates are ambiguous, so report missing")
}

// TestMatchesSubset_ArrayAppendTrap is the H1 regression trap. We
// deliberately pin the current strict-array behavior (server-side
// appended element means drift. If this assumption breaks, the
// Akuity gateway starts injecting defaulted array members into a
// supported declarative child, and the test name points to the right
// design decision in drift.go and gives reviewers a single place to
// flip if per-kind keyed matching is the answer.
//
// The scenario: user declares one container; the server "helpfully"
// appends a sidecar. Today that is drift. If we ever relax arrays to
// prefix-match or keyed-match, this test must be rewritten deliberately,
// not accidentally.
func TestMatchesSubset_ArrayAppendTrap(t *testing.T) {
	desired, err := Index([]runtime.RawExtension{
		rawOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app"},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "main", "image": "nginx"},
				},
			},
		}),
	})
	require.NoError(t, err)

	observed, err := IndexStructs([]*structpb.Struct{
		structOf(t, map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "app"},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "main", "image": "nginx"},
					map[string]interface{}{"name": "sidecar", "image": "defaulted"},
				},
			},
		}),
	})
	require.NoError(t, err)

	report := Compare(desired, observed)
	assert.Len(t, report.Changed, 1,
		"server-appended array element currently fires drift; if the gateway starts doing this, revisit matchesSubset's array rule and consider per-kind keyed matching")
}

// TestMatchesSubset_NestedMapDivergence asserts that divergent scalar
// values under a shared path fire a mismatch even when surrounding
// structure is equivalent.
func TestMatchesSubset_NestedMapDivergence(t *testing.T) {
	desired := map[string]interface{}{
		"spec": map[string]interface{}{
			"source": map[string]interface{}{"path": "a"},
		},
	}
	observedSame := map[string]interface{}{
		"spec": map[string]interface{}{
			"source": map[string]interface{}{"path": "a", "repoURL": "https://x"},
		},
	}
	assert.True(t, matchesSubset(desired, observedSame), "extra nested field on observed is ignored")

	observedDiff := map[string]interface{}{
		"spec": map[string]interface{}{
			"source": map[string]interface{}{"path": "b"},
		},
	}
	assert.False(t, matchesSubset(desired, observedDiff), "divergent path fires mismatch")
}

// TestMatchesSubset_EmptyValueAbsentEquivalence locks in the
// "spec: {}" / metadata-only Kargo Project case: a user-declared
// empty map / empty list / nil under a key MUST match an observed
// payload that omits the key entirely. Without this equivalence, the
// gateway's habit of suppressing empty containers from its export
// response (e.g. Kargo Project echoes back metadata only when no
// status fields are populated) flags drift on every reconcile and
// hot-loops the Apply path.
func TestMatchesSubset_EmptyValueAbsentEquivalence(t *testing.T) {
	cases := []struct {
		name     string
		desired  map[string]interface{}
		observed map[string]interface{}
		match    bool
	}{
		{
			name: "desired empty map vs observed key absent",
			desired: map[string]interface{}{
				"apiVersion": "kargo.akuity.io/v1alpha1",
				"kind":       "Project",
				"metadata":   map[string]interface{}{"name": "p1"},
				"spec":       map[string]interface{}{},
			},
			observed: map[string]interface{}{
				"apiVersion": "kargo.akuity.io/v1alpha1",
				"kind":       "Project",
				"metadata":   map[string]interface{}{"name": "p1", "finalizers": []interface{}{"kargo.akuity.io/finalizer"}},
			},
			match: true,
		},
		{
			name: "desired empty list vs observed key absent",
			desired: map[string]interface{}{
				"spec": map[string]interface{}{"items": []interface{}{}},
			},
			observed: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			match: true,
		},
		{
			name: "desired nil vs observed key absent",
			desired: map[string]interface{}{
				"spec": map[string]interface{}{"x": nil},
			},
			observed: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			match: true,
		},
		{
			name: "desired non-empty map still requires presence",
			desired: map[string]interface{}{
				"spec": map[string]interface{}{"x": float64(1)},
			},
			observed: map[string]interface{}{},
			match:    false,
		},
		{
			name: "desired all-empty values vs nil observed map",
			desired: map[string]interface{}{
				"top": map[string]interface{}{},
			},
			observed: nil,
			// nil observed of declared type map[string]interface{}
			// behaves like an empty map under range; an all-empty
			// desired therefore matches symmetrically with the
			// per-key absent-equivalence rule above.
			match: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := matchesSubset(tc.desired, tc.observed)
			assert.Equal(t, tc.match, got, "matchesSubset diverged from expected")
		})
	}
}
