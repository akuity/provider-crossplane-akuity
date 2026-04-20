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

package instanceipallowlist

import (
	"context"
	"testing"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
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

// newAllowListByRef builds an MR that resolves the instance via a
// same-namespace InstanceRef. The underlying Instance MR reports
// ID=inst-1 in its Status, which the controller picks up.
func newAllowListByRef() *v1alpha2.InstanceIpAllowList {
	return &v1alpha2.InstanceIpAllowList{
		ObjectMeta: metav1.ObjectMeta{Name: "allow", Namespace: "ns"},
		Spec: v1alpha2.InstanceIpAllowListSpec{
			ForProvider: v1alpha2.InstanceIpAllowListParameters{
				InstanceRef: &v1alpha2.LocalReference{Name: "inst"},
				AllowList: []*v1alpha2.IPAllowListEntry{
					{Ip: "10.0.0.1", Description: "office"},
				},
			},
		},
	}
}

// newAllowListByID builds an MR that supplies the Akuity instance ID
// directly — the controller must not need the kube client at all.
func newAllowListByID() *v1alpha2.InstanceIpAllowList {
	return &v1alpha2.InstanceIpAllowList{
		ObjectMeta: metav1.ObjectMeta{Name: "allow", Namespace: "ns"},
		Spec: v1alpha2.InstanceIpAllowListSpec{
			ForProvider: v1alpha2.InstanceIpAllowListParameters{
				InstanceID: "inst-1",
				AllowList: []*v1alpha2.IPAllowListEntry{
					{Ip: "10.0.0.1", Description: "office"},
				},
			},
		},
	}
}

func newInst() *v1alpha2.Instance {
	return &v1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "ns"},
		Spec: v1alpha2.InstanceSpec{
			ForProvider: v1alpha2.InstanceParameters{
				Name: "inst",
				ArgoCD: &v1alpha2.ArgoCD{
					Spec: v1alpha2.ArgoCDSpec{Version: "v2.13.0"},
				},
			},
		},
		Status: v1alpha2.InstanceStatus{AtProvider: v1alpha2.InstanceObservation{ID: "inst-1"}},
	}
}

func newExt(t *testing.T, objs ...runtime.Object) (*external, *mockclient.MockClient) {
	t.Helper()
	mc := mockclient.NewMockClient(gomock.NewController(t))
	kube := fake.NewClientBuilder().WithScheme(newScheme(t)).WithRuntimeObjects(objs...).Build()
	return &external{ExternalClient: base.ExternalClient{Client: mc, Kube: kube, Logger: logging.NewNopLogger()}}, mc
}

func TestObserve_NoExternalName(t *testing.T) {
	e, _ := newExt(t, newInst())
	obs, err := e.Observe(context.Background(), newAllowListByRef())
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_ProjectsCurrentAllowList_ByRef(t *testing.T) {
	e, mc := newExt(t, newInst())
	al := newAllowListByRef()
	meta.SetExternalName(al, al.Name)

	mc.EXPECT().GetInstanceByID(gomock.Any(), "inst-1").Return(&argocdv1.Instance{
		Id:   "inst-1",
		Name: "inst",
		Spec: &argocdv1.InstanceSpec{
			IpAllowList: []*argocdv1.IPAllowListEntry{{Ip: "10.0.0.1", Description: "office"}},
		},
	}, nil).Times(1)

	obs, err := e.Observe(context.Background(), al)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.True(t, obs.ResourceUpToDate)
	require.Len(t, al.Status.AtProvider.AllowList, 1)
	assert.Equal(t, "10.0.0.1", al.Status.AtProvider.AllowList[0].Ip)
}

func TestObserve_ByIDSkipsKubeLookup(t *testing.T) {
	// No Instance MR staged; the MR carries the ID on its spec so the
	// controller must never touch the kube client.
	e, mc := newExt(t)
	al := newAllowListByID()
	meta.SetExternalName(al, al.Name)

	mc.EXPECT().GetInstanceByID(gomock.Any(), "inst-1").Return(&argocdv1.Instance{
		Id: "inst-1",
		Spec: &argocdv1.InstanceSpec{
			IpAllowList: []*argocdv1.IPAllowListEntry{{Ip: "10.0.0.1", Description: "office"}},
		},
	}, nil).Times(1)

	obs, err := e.Observe(context.Background(), al)
	require.NoError(t, err)
	assert.True(t, obs.ResourceUpToDate)
}

func TestObserve_DriftDetected(t *testing.T) {
	e, mc := newExt(t, newInst())
	al := newAllowListByRef()
	meta.SetExternalName(al, al.Name)

	mc.EXPECT().GetInstanceByID(gomock.Any(), "inst-1").Return(&argocdv1.Instance{
		Spec: &argocdv1.InstanceSpec{
			IpAllowList: []*argocdv1.IPAllowListEntry{{Ip: "10.0.0.2", Description: "other"}},
		},
	}, nil).Times(1)

	obs, err := e.Observe(context.Background(), al)
	require.NoError(t, err)
	assert.False(t, obs.ResourceUpToDate)
}

func TestCreate_PatchInstanceIsCalled(t *testing.T) {
	e, mc := newExt(t, newInst())
	al := newAllowListByRef()

	// Capture the patch struct so we can assert the narrow sub-tree
	// shape: { spec: { ipAllowList: [{ip, description}] } } — nothing else.
	var captured *structpb.Struct
	mc.EXPECT().PatchInstance(gomock.Any(), "inst-1", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, p *structpb.Struct) error {
		captured = p
		return nil
	}).Times(1)

	_, err := e.Create(context.Background(), al)
	require.NoError(t, err)
	assert.Equal(t, al.Name, meta.GetExternalName(al))

	require.NotNil(t, captured)
	m := captured.AsMap()
	spec, ok := m["spec"].(map[string]any)
	require.True(t, ok, "patch missing spec: %v", m)
	ipList, ok := spec["ipAllowList"].([]any)
	require.True(t, ok, "spec missing ipAllowList: %v", spec)
	require.Len(t, ipList, 1)
	entry := ipList[0].(map[string]any)
	assert.Equal(t, "10.0.0.1", entry["ip"])
	assert.Equal(t, "office", entry["description"])
}

func TestDelete_ClearsAllowList(t *testing.T) {
	e, mc := newExt(t, newInst())
	al := newAllowListByRef()
	meta.SetExternalName(al, al.Name)

	var captured *structpb.Struct
	mc.EXPECT().PatchInstance(gomock.Any(), "inst-1", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, p *structpb.Struct) error {
		captured = p
		return nil
	}).Times(1)

	_, err := e.Delete(context.Background(), al)
	require.NoError(t, err)

	require.NotNil(t, captured)
	m := captured.AsMap()
	spec := m["spec"].(map[string]any)
	ipList, ok := spec["ipAllowList"].([]any)
	require.True(t, ok)
	assert.Empty(t, ipList, "delete must emit an empty list, not a missing key")
}

func TestResolveInstanceID_RefWithoutID_Errors(t *testing.T) {
	// Instance MR exists but its Status.AtProvider.ID is empty — the
	// controller should error instead of silently patching with an empty
	// string.
	pendingInst := newInst()
	pendingInst.Status.AtProvider.ID = ""
	e, _ := newExt(t, pendingInst)

	al := newAllowListByRef()
	_, err := e.resolveInstanceID(context.Background(), al)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has not yet reported an ID")
}
