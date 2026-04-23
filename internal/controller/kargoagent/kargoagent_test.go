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
	"context"
	"testing"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	reconv1 "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	mockclient "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

func newAgent() *v1alpha1.KargoAgent {
	return &v1alpha1.KargoAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "agt", Namespace: "ns"},
		Spec: v1alpha1.KargoAgentSpec{
			ForProvider: v1alpha1.KargoAgentParameters{
				KargoInstanceID: "ki-1",
				Name:            "agt",
				Data:            crossplanetypes.KargoAgentData{Size: crossplanetypes.KargoAgentSize("small")},
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
	obs, err := e.Observe(context.Background(), newAgent())
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_NotFound(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")
	mc.EXPECT().GetKargoInstanceAgent(gomock.Any(), "ki-1", "agt").
		Return(nil, reason.AsNotFound(errors.New("no"))).Times(1)
	obs, err := e.Observe(context.Background(), a)
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_NotYetReconciled(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")
	mc.EXPECT().GetKargoInstanceAgent(gomock.Any(), "ki-1", "agt").Return(&kargov1.KargoAgent{
		Id:                   "ag-1",
		Name:                 "agt",
		ReconciliationStatus: &reconv1.Status{Code: reconv1.StatusCode_STATUS_CODE_PROGRESSING},
	}, nil).Times(1)
	// Agent is still provisioning — not yet in Export's Agents slice.
	// Observe falls back to Get-derived drift, which surfaces the
	// (empty Data vs desired small) diff.
	mc.EXPECT().ExportKargoInstance(gomock.Any(), "ki-1").
		Return(&kargov1.ExportKargoInstanceResponse{}, nil).Times(1)

	obs, err := e.Observe(context.Background(), a)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.False(t, obs.ResourceUpToDate)
	assert.Nil(t, obs.ConnectionDetails)
}

func TestObserve_ReconciledPublishesManifests(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")
	mc.EXPECT().GetKargoInstanceAgent(gomock.Any(), "ki-1", "agt").Return(&kargov1.KargoAgent{
		Id:                   "ag-1",
		Name:                 "agt",
		Data:                 &kargov1.KargoAgentData{},
		ReconciliationStatus: &reconv1.Status{Code: reconv1.StatusCode_STATUS_CODE_SUCCESSFUL},
		HealthStatus:         &health.Status{Code: health.StatusCode_STATUS_CODE_HEALTHY},
	}, nil).Times(1)
	// Manifests-publishing test does not assert drift status; return an
	// empty Agents list so we exercise the Get-fallback path.
	mc.EXPECT().ExportKargoInstance(gomock.Any(), "ki-1").
		Return(&kargov1.ExportKargoInstanceResponse{}, nil).Times(1)
	mc.EXPECT().GetKargoInstanceAgentManifestsOnce(gomock.Any(), "ki-1", "ag-1").
		Return("kind: ConfigMap\n", nil).Times(1)

	obs, err := e.Observe(context.Background(), a)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	require.NotNil(t, obs.ConnectionDetails)
	assert.Equal(t, []byte("kind: ConfigMap\n"), obs.ConnectionDetails[ConnectionKeyManifests])
}

func TestCreate_Apply(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	mc.EXPECT().CreateKargoInstanceAgent(gomock.Any(), gomock.Any()).
		Return(&kargov1.KargoAgent{Id: "ag-1", Name: "agt"}, nil).Times(1)
	_, err := e.Create(context.Background(), a)
	require.NoError(t, err)
	assert.Equal(t, "agt", meta.GetExternalName(a))
}

func TestDelete_CallsDelete(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")
	mc.EXPECT().DeleteKargoInstanceAgent(gomock.Any(), "ki-1", "agt").Return(nil).Times(1)
	_, err := e.Delete(context.Background(), a)
	require.NoError(t, err)
}

func TestDelete_ConnectedAgentError_WrapsRetryable(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")
	mc.EXPECT().DeleteKargoInstanceAgent(gomock.Any(), "ki-1", "agt").
		Return(errors.New("cannot delete: there are connected agent clusters")).Times(1)
	_, err := e.Delete(context.Background(), a)
	require.Error(t, err)
	assert.True(t, reason.IsRetryable(err))
}
