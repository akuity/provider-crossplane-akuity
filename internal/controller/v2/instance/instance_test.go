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
	"testing"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	mockclient "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

func newInstance() *v1alpha2.Instance {
	return &v1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst", Namespace: "ns"},
		Spec: v1alpha2.InstanceSpec{
			ForProvider: v1alpha2.InstanceParameters{
				Name: "inst",
				ArgoCD: &v1alpha2.ArgoCDSpec{Version: "v2.13.0"},
			},
		},
	}
}

func newExternal(t *testing.T) (*external, *mockclient.MockClient) {
	t.Helper()
	mc := mockclient.NewMockClient(gomock.NewController(t))
	return &external{ExternalClient: base.ExternalClient{Client: mc, Logger: logging.NewNopLogger()}}, mc
}

func TestObserve_NoExternalName(t *testing.T) {
	e, _ := newExternal(t)
	obs, err := e.Observe(context.Background(), newInstance())
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_NotFoundMapsToResourceExistsFalse(t *testing.T) {
	e, mc := newExternal(t)
	inst := newInstance()
	meta.SetExternalName(inst, "inst")

	mc.EXPECT().GetInstance(gomock.Any(), "inst").
		Return(nil, reason.AsNotFound(errors.New("nope"))).Times(1)

	obs, err := e.Observe(context.Background(), inst)
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_PropagatesGenericErr(t *testing.T) {
	e, mc := newExternal(t)
	inst := newInstance()
	meta.SetExternalName(inst, "inst")

	mc.EXPECT().GetInstance(gomock.Any(), "inst").
		Return(nil, errors.New("boom")).Times(1)

	_, err := e.Observe(context.Background(), inst)
	require.Error(t, err)
}

func TestObserve_Available(t *testing.T) {
	e, mc := newExternal(t)
	inst := newInstance()
	meta.SetExternalName(inst, "inst")

	ai := &argocdv1.Instance{
		Id:      "id-1",
		Name:    "inst",
		Version: "v2.13.0",
		HealthStatus: &health.Status{
			Code: health.StatusCode_STATUS_CODE_HEALTHY,
		},
	}

	mc.EXPECT().GetInstance(gomock.Any(), "inst").Return(ai, nil).Times(1)
	mc.EXPECT().ExportInstance(gomock.Any(), "inst").Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	obs, err := e.Observe(context.Background(), inst)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.Equal(t, "id-1", inst.Status.AtProvider.ID)
	assert.Equal(t, int32(health.StatusCode_STATUS_CODE_HEALTHY), inst.Status.AtProvider.HealthStatus.Code)
}

func TestCreate_SetsExternalName(t *testing.T) {
	e, mc := newExternal(t)
	inst := newInstance()

	mc.EXPECT().ApplyInstance(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	resp, err := e.Create(context.Background(), inst)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
	assert.Equal(t, "inst", meta.GetExternalName(inst))
}

func TestUpdate_CallsApply(t *testing.T) {
	e, mc := newExternal(t)
	inst := newInstance()

	mc.EXPECT().ApplyInstance(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	_, err := e.Update(context.Background(), inst)
	require.NoError(t, err)
}

func TestDelete_NoExternalNameNoOp(t *testing.T) {
	e, _ := newExternal(t)
	_, err := e.Delete(context.Background(), newInstance())
	require.NoError(t, err)
}

func TestDelete_CallsDeleteInstance(t *testing.T) {
	e, mc := newExternal(t)
	inst := newInstance()
	meta.SetExternalName(inst, "inst")

	mc.EXPECT().DeleteInstance(gomock.Any(), "inst").Return(nil).Times(1)

	_, err := e.Delete(context.Background(), inst)
	require.NoError(t, err)
}
