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
	"testing"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
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

func newKI() *v1alpha2.KargoInstance {
	return &v1alpha2.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki", Namespace: "ns"},
		Spec: v1alpha2.KargoInstanceResourceSpec{
			ForProvider: v1alpha2.KargoInstanceParameters{
				Name: "ki",
				Spec: v1alpha2.KargoSpec{Version: "v1.0.0"},
			},
		},
	}
}

func newExt(t *testing.T) (*external, *mockclient.MockClient) {
	t.Helper()
	mc := mockclient.NewMockClient(gomock.NewController(t))
	return &external{ExternalClient: base.ExternalClient{Client: mc, Logger: logging.NewNopLogger()}}, mc
}

func TestObserve_NoExternalName(t *testing.T) {
	e, _ := newExt(t)
	obs, err := e.Observe(context.Background(), newKI())
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_NotFound(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().GetKargoInstance(gomock.Any(), "ki").Return(nil, reason.AsNotFound(errors.New("no"))).Times(1)
	obs, err := e.Observe(context.Background(), ki)
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_Available(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().GetKargoInstance(gomock.Any(), "ki").Return(&kargov1.KargoInstance{
		Id:           "id-1",
		Name:         "ki",
		Version:      "v1.0.0",
		HealthStatus: &health.Status{Code: health.StatusCode_STATUS_CODE_HEALTHY},
	}, nil).Times(1)

	obs, err := e.Observe(context.Background(), ki)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.Equal(t, "id-1", ki.Status.AtProvider.ID)
}

func TestCreate_Apply(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	_, err := e.Create(context.Background(), ki)
	require.NoError(t, err)
	assert.Equal(t, "ki", meta.GetExternalName(ki))
}

func TestDelete_CallsDelete(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().DeleteKargoInstance(gomock.Any(), "ki").Return(nil).Times(1)
	_, err := e.Delete(context.Background(), ki)
	require.NoError(t, err)
}
