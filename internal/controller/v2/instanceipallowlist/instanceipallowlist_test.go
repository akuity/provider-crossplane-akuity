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

func newAllowList() *v1alpha2.InstanceIpAllowList {
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
	obs, err := e.Observe(context.Background(), newAllowList())
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_ProjectsCurrentAllowList(t *testing.T) {
	e, mc := newExt(t, newInst())
	al := newAllowList()
	meta.SetExternalName(al, al.Name)

	mc.EXPECT().GetInstance(gomock.Any(), "inst").Return(&argocdv1.Instance{
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

func TestObserve_DriftDetected(t *testing.T) {
	e, mc := newExt(t, newInst())
	al := newAllowList()
	meta.SetExternalName(al, al.Name)

	mc.EXPECT().GetInstance(gomock.Any(), "inst").Return(&argocdv1.Instance{
		Spec: &argocdv1.InstanceSpec{
			IpAllowList: []*argocdv1.IPAllowListEntry{{Ip: "10.0.0.2", Description: "other"}},
		},
	}, nil).Times(1)

	obs, err := e.Observe(context.Background(), al)
	require.NoError(t, err)
	assert.False(t, obs.ResourceUpToDate)
}

func TestCreate_ApplyInstanceIsCalled(t *testing.T) {
	e, mc := newExt(t, newInst())
	al := newAllowList()

	mc.EXPECT().GetInstance(gomock.Any(), "inst").Return(&argocdv1.Instance{
		Name: "inst",
		Spec: &argocdv1.InstanceSpec{},
	}, nil).Times(1)
	mc.EXPECT().ApplyInstance(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	_, err := e.Create(context.Background(), al)
	require.NoError(t, err)
	assert.Equal(t, al.Name, meta.GetExternalName(al))
}

func TestDelete_ClearsAllowList(t *testing.T) {
	e, mc := newExt(t, newInst())
	al := newAllowList()
	meta.SetExternalName(al, al.Name)

	mc.EXPECT().GetInstance(gomock.Any(), "inst").Return(&argocdv1.Instance{
		Name: "inst",
		Spec: &argocdv1.InstanceSpec{},
	}, nil).Times(1)
	mc.EXPECT().ApplyInstance(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	_, err := e.Delete(context.Background(), al)
	require.NoError(t, err)
}
