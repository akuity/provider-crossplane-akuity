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

package kargoagent

import (
	"testing"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// TestApiToSpec_CarriesSpecOnlyFields verifies the key contract on
// apiToSpec: fields the user owns locally (KargoInstanceID / Ref /
// Workspace) must come from the *desired* struct, because the Akuity
// API has no notion of the Crossplane MR's parent reference.
func TestApiToSpec_CarriesSpecOnlyFields(t *testing.T) {
	desired := v1alpha1.KargoAgentParameters{
		KargoInstanceID:  "ki-1",
		KargoInstanceRef: &v1alpha1.LocalReference{Name: "kiref"},
		Workspace:        "ws-1",
	}
	agent := &kargov1.KargoAgent{
		Id:          "ag-1",
		Name:        "agt",
		Description: "observed-description",
		Data: &kargov1.KargoAgentData{
			Size:                KargoAgentSizeProto("small"),
			Namespace:           "kargo",
			Labels:              map[string]string{"team": "platform"},
			Annotations:         map[string]string{"note": "x"},
			AutoUpgradeDisabled: boolPtr(true),
		},
	}

	out := apiToSpec(desired, agent)
	assert.Equal(t, "ki-1", out.KargoInstanceID)
	assert.Equal(t, "kiref", out.KargoInstanceRef.Name)
	assert.Equal(t, "ws-1", out.Workspace)
	assert.Equal(t, "agt", out.Name, "Name must come from API (immutable by CEL)")
	assert.Equal(t, "kargo", out.Namespace, "Namespace is pulled from wire Data, not ObjectMeta")
	assert.Equal(t, "observed-description", out.Description)
	assert.Equal(t, map[string]string{"team": "platform"}, out.Labels)
	assert.NotEmpty(t, out.Data.Size, "size must roundtrip through the bridge")
	require.NotNil(t, out.Data.AutoUpgradeDisabled)
	assert.True(t, *out.Data.AutoUpgradeDisabled)
}

// TestApiToSpec_NilDataDoesNotPanic covers the boundary: an agent
// still in provisioning may have no Data block yet. Observe must not
// panic and must leave Data at its zero value.
func TestApiToSpec_NilDataDoesNotPanic(t *testing.T) {
	desired := v1alpha1.KargoAgentParameters{KargoInstanceID: "ki-1"}
	agent := &kargov1.KargoAgent{Id: "ag-1", Name: "agt"}

	out := apiToSpec(desired, agent)
	assert.Equal(t, "agt", out.Name)
	assert.Empty(t, out.Namespace)
	assert.Equal(t, crossplanetypes.KargoAgentSize(""), out.Data.Size)
}

// TestWireToSpec_NilReturnsZero covers the explicit nil guard so
// Observe never feeds a nil wire into drift detection.
func TestWireToSpec_NilReturnsZero(t *testing.T) {
	out := wireToSpec(v1alpha1.KargoAgentParameters{}, nil)
	assert.Equal(t, v1alpha1.KargoAgentParameters{}, out)
}

// TestWireToSpec_PullsMetadataFromObjectMeta locks in the wire-shape
// difference from the proto path: Namespace / Labels / Annotations
// live on ObjectMeta, not on Data. A regression here would drop those
// fields from the subset-drift comparison.
func TestWireToSpec_PullsMetadataFromObjectMeta(t *testing.T) {
	desired := v1alpha1.KargoAgentParameters{
		KargoInstanceID: "ki-1",
		Workspace:       "ws-1",
	}
	wire := &akuitytypes.KargoAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "agt",
			Namespace:   "kargo",
			Labels:      map[string]string{"team": "platform"},
			Annotations: map[string]string{"note": "x"},
		},
		Spec: akuitytypes.KargoAgentSpec{
			Description: "observed",
			Data:        akuitytypes.KargoAgentData{Size: akuitytypes.KargoAgentSize("medium")},
		},
	}

	out := wireToSpec(desired, wire)
	assert.Equal(t, "ki-1", out.KargoInstanceID)
	assert.Equal(t, "ws-1", out.Workspace)
	assert.Equal(t, "agt", out.Name)
	assert.Equal(t, "kargo", out.Namespace)
	assert.Equal(t, map[string]string{"team": "platform"}, out.Labels)
	assert.Equal(t, map[string]string{"note": "x"}, out.Annotations)
	assert.Equal(t, "observed", out.Description)
	assert.Equal(t, crossplanetypes.KargoAgentSize("medium"), out.Data.Size)
}

// TestWireToSpec_EmptyLabelMapsBecomeNil exercises the nil-vs-empty
// normalisation in wireToSpec: the Akuity wire carries an empty map,
// the desired spec uses nil for "absent", so the comparison path needs
// them collapsed to nil to avoid false drift.
func TestWireToSpec_EmptyLabelMapsBecomeNil(t *testing.T) {
	wire := &akuitytypes.KargoAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "agt",
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	out := wireToSpec(v1alpha1.KargoAgentParameters{}, wire)
	assert.Nil(t, out.Labels, "empty labels map must collapse to nil so drift doesn't fire")
	assert.Nil(t, out.Annotations)
}

// TestBuildApplyKargoInstanceRequest_InjectsNamespaceAndLabels covers
// the Apply-path round-trip: the akuity wire KargoAgent carries
// Namespace / Labels / Annotations on ObjectMeta (unlike the proto
// KargoAgentData which carries them inline), so SpecToAPI must emit
// them there for ApplyKargoInstance to round-trip with Export.
func TestBuildApplyKargoInstanceRequest_InjectsNamespaceAndLabels(t *testing.T) {
	p := v1alpha1.KargoAgentParameters{
		Name:        "agt",
		Namespace:   "kargo",
		Labels:      map[string]string{"team": "platform"},
		Annotations: map[string]string{"note": "x"},
		Data: crossplanetypes.KargoAgentData{
			Size: crossplanetypes.KargoAgentSize("small"),
		},
	}
	req, err := BuildApplyKargoInstanceRequest("ki-1", p)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Len(t, req.GetAgents(), 1)
	meta, ok := req.GetAgents()[0].AsMap()["metadata"].(map[string]any)
	require.True(t, ok, "agents[0].metadata present")
	assert.Equal(t, "kargo", meta["namespace"])
	assert.Equal(t, map[string]any{"team": "platform"}, meta["labels"])
	assert.Equal(t, map[string]any{"note": "x"}, meta["annotations"])
}

// TestBuildApplyKargoInstanceRequest_InvalidKustomizationErrors keeps
// the validation guard on Kustomization — users pass raw YAML, and a
// malformed kustomization.yaml must fail at encode rather than be
// silently forwarded to the gateway.
func TestBuildApplyKargoInstanceRequest_InvalidKustomizationErrors(t *testing.T) {
	p := v1alpha1.KargoAgentParameters{
		Data: crossplanetypes.KargoAgentData{
			Kustomization: "this: is: not: yaml:\n    - [",
		},
	}
	_, err := BuildApplyKargoInstanceRequest("ki-1", p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kustomization")
}

// TestConvertAgentData_NilReturnsNil covers the explicit nil guard.
func TestConvertAgentData_NilReturnsNil(t *testing.T) {
	assert.Nil(t, convertAgentData(nil))
}

// TestConvertAgentData_PropagatesSize exercises the roundtrip from
// proto → wire → spec for the most user-visible field (Size). The
// exact string form varies with how the marshal bridge renders enums;
// what matters here is that a non-zero enum on the proto produces a
// non-empty Size on the spec (wire drift would silently zero it out).
func TestConvertAgentData_PropagatesSize(t *testing.T) {
	pb := &kargov1.KargoAgentData{Size: KargoAgentSizeProto("small")}
	out := convertAgentData(pb)
	require.NotNil(t, out)
	assert.NotEmpty(t, out.Size, "wire drift would drop Size during the bridge")
}

// TestIsConnectedAgentDeleteError_Substring locks the substring match
// that drives Delete's retryable wrap. If the gateway phrasing drifts
// the controller needs to know so it can update the guard.
func TestIsConnectedAgentDeleteError_Substring(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"cannot delete: there are connected agent clusters", true},
		{"connected agent clusters still present", true},
		{"not found", false},
		{"permission denied", false},
	}
	for _, tc := range cases {
		got := isConnectedAgentDeleteError(errString(tc.in))
		assert.Equal(t, tc.want, got, "phrase %q", tc.in)
	}
}

// KargoAgentSizeProto is a tiny helper so the proto-typed literal
// reads nicely in test fixtures.
func KargoAgentSizeProto(s string) kargov1.KargoAgentSize {
	switch s {
	case "small":
		return kargov1.KargoAgentSize_KARGO_AGENT_SIZE_SMALL
	case "medium":
		return kargov1.KargoAgentSize_KARGO_AGENT_SIZE_MEDIUM
	case "large":
		return kargov1.KargoAgentSize_KARGO_AGENT_SIZE_LARGE
	default:
		return kargov1.KargoAgentSize_KARGO_AGENT_SIZE_UNSPECIFIED
	}
}

func boolPtr(b bool) *bool { return &b }

type errString string

func (e errString) Error() string { return string(e) }
