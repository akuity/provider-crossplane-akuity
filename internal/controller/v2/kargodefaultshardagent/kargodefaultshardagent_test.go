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

package kargodefaultshardagent

import (
	"context"
	"testing"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	mockclient "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, v1alpha2.SchemeBuilder.AddToScheme(s))
	return s
}

func newDSAByRef() *v1alpha2.KargoDefaultShardAgent {
	return &v1alpha2.KargoDefaultShardAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "dsa", Namespace: "ns"},
		Spec: v1alpha2.KargoDefaultShardAgentSpec{
			ForProvider: v1alpha2.KargoDefaultShardAgentParameters{
				KargoInstanceRef: &v1alpha2.LocalReference{Name: "ki"},
				AgentName:        "shard-a",
			},
		},
	}
}

func newDSAByID() *v1alpha2.KargoDefaultShardAgent {
	return &v1alpha2.KargoDefaultShardAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "dsa", Namespace: "ns"},
		Spec: v1alpha2.KargoDefaultShardAgentSpec{
			ForProvider: v1alpha2.KargoDefaultShardAgentParameters{
				KargoInstanceID: "ki-1",
				AgentName:       "shard-a",
			},
		},
	}
}

func newKI() *v1alpha2.KargoInstance {
	return &v1alpha2.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki", Namespace: "ns"},
		Spec: v1alpha2.KargoInstanceResourceSpec{
			ForProvider: v1alpha2.KargoInstanceParameters{
				Name: "ki",
				Kargo: v1alpha2.KargoSpec{Version: "v1.0.0"},
			},
		},
		Status: v1alpha2.KargoInstanceStatus{AtProvider: v1alpha2.KargoInstanceObservation{ID: "ki-1"}},
	}
}

func newExt(t *testing.T, objs ...runtime.Object) (*external, *mockclient.MockClient) {
	t.Helper()
	mc := mockclient.NewMockClient(gomock.NewController(t))
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithRuntimeObjects(objs...).Build()
	return &external{ExternalClient: base.ExternalClient{Client: mc, Kube: kube, Logger: logging.NewNopLogger()}}, mc
}

func TestObserve_NoExternalName(t *testing.T) {
	e, _ := newExt(t, newKI())
	obs, err := e.Observe(context.Background(), newDSAByRef())
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_UpToDate_ByRef(t *testing.T) {
	e, mc := newExt(t, newKI())
	dsa := newDSAByRef()
	meta.SetExternalName(dsa, dsa.Name)
	mc.EXPECT().GetKargoInstanceByID(gomock.Any(), "ki-1").Return(&kargov1.KargoInstance{
		Id:   "ki-1",
		Name: "ki",
		Spec: &kargov1.KargoInstanceSpec{DefaultShardAgent: "shard-a"},
	}, nil).Times(1)

	obs, err := e.Observe(context.Background(), dsa)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.True(t, obs.ResourceUpToDate)
	assert.Equal(t, "shard-a", dsa.Status.AtProvider.AgentName)
}

func TestObserve_ByIDSkipsKubeLookup(t *testing.T) {
	e, mc := newExt(t) // no KargoInstance MR staged
	dsa := newDSAByID()
	meta.SetExternalName(dsa, dsa.Name)
	mc.EXPECT().GetKargoInstanceByID(gomock.Any(), "ki-1").Return(&kargov1.KargoInstance{
		Id:   "ki-1",
		Spec: &kargov1.KargoInstanceSpec{DefaultShardAgent: "shard-a"},
	}, nil).Times(1)

	obs, err := e.Observe(context.Background(), dsa)
	require.NoError(t, err)
	assert.True(t, obs.ResourceUpToDate)
}

func TestObserve_Drift(t *testing.T) {
	e, mc := newExt(t, newKI())
	dsa := newDSAByRef()
	meta.SetExternalName(dsa, dsa.Name)
	mc.EXPECT().GetKargoInstanceByID(gomock.Any(), "ki-1").Return(&kargov1.KargoInstance{
		Id:   "ki-1",
		Spec: &kargov1.KargoInstanceSpec{DefaultShardAgent: "shard-b"},
	}, nil).Times(1)

	obs, err := e.Observe(context.Background(), dsa)
	require.NoError(t, err)
	assert.False(t, obs.ResourceUpToDate)
}

func TestCreate_PatchIsCalled(t *testing.T) {
	e, mc := newExt(t, newKI())
	dsa := newDSAByRef()

	var captured *structpb.Struct
	mc.EXPECT().PatchKargoInstance(gomock.Any(), "ki-1", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, p *structpb.Struct) error {
		captured = p
		return nil
	}).Times(1)

	_, err := e.Create(context.Background(), dsa)
	require.NoError(t, err)
	assert.Equal(t, dsa.Name, meta.GetExternalName(dsa))

	require.NotNil(t, captured)
	m := captured.AsMap()
	spec := m["spec"].(map[string]any)
	kis := spec["kargoInstanceSpec"].(map[string]any)
	assert.Equal(t, "shard-a", kis["defaultShardAgent"])
}

func TestDelete_ClearsDefault(t *testing.T) {
	e, mc := newExt(t, newKI())
	dsa := newDSAByRef()
	meta.SetExternalName(dsa, dsa.Name)

	var captured *structpb.Struct
	mc.EXPECT().PatchKargoInstance(gomock.Any(), "ki-1", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, p *structpb.Struct) error {
		captured = p
		return nil
	}).Times(1)

	_, err := e.Delete(context.Background(), dsa)
	require.NoError(t, err)

	require.NotNil(t, captured)
	m := captured.AsMap()
	spec := m["spec"].(map[string]any)
	kis := spec["kargoInstanceSpec"].(map[string]any)
	assert.Empty(t, kis["defaultShardAgent"])
}

func TestResolveKargoID_RefWithoutID_Errors(t *testing.T) {
	pendingKI := newKI()
	pendingKI.Status.AtProvider.ID = ""
	e, _ := newExt(t, pendingKI)
	_, err := e.resolveKargoID(context.Background(), newDSAByRef())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has not yet reported an ID")
}
