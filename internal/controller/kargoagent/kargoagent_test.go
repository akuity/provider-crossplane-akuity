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
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	mockclient "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// provisioningWaitErr synthesises the gRPC error shape the Akuity
// gateway returns while a target resource is still being provisioned.
// reason.IsProvisioningWait keys off codes.InvalidArgument + the
// "still being provisioned" substring.
func provisioningWaitErr() error {
	return status.Error(codes.InvalidArgument, "instance still being provisioned")
}

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

	var capturedReq *kargov1.ApplyKargoInstanceRequest
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *kargov1.ApplyKargoInstanceRequest) error {
		capturedReq = req
		return nil
	}).Times(1)

	_, err := e.Create(context.Background(), a)
	require.NoError(t, err)
	assert.Equal(t, "agt", meta.GetExternalName(a), "externalName is the user-supplied agent name; Apply has no response body")
	require.NotNil(t, capturedReq)
	assert.Equal(t, "ki-1", capturedReq.GetId())
	require.Len(t, capturedReq.GetAgents(), 1, "narrow-merge: only Agents populated, sibling fields left for KargoInstance MR")
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

// TestDelete_EmptyExternalName short-circuits before the gateway call
// so Crossplane can release the finalizer on MRs that never got an
// external name (e.g. Create failed before SetExternalName).
func TestDelete_EmptyExternalName(t *testing.T) {
	e, _ := newExt(t)
	a := newAgent()
	// external-name deliberately unset.
	_, err := e.Delete(context.Background(), a)
	require.NoError(t, err)
}

// TestDelete_GenericErrPropagates: a non-retryable gateway error on
// Delete must surface so managed.Reconciler backoff-requeues.
func TestDelete_GenericErrPropagates(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")
	mc.EXPECT().DeleteKargoInstanceAgent(gomock.Any(), "ki-1", "agt").
		Return(errors.New("boom")).Times(1)
	_, err := e.Delete(context.Background(), a)
	require.Error(t, err)
	assert.False(t, reason.IsRetryable(err), "generic error must not be wrapped as retryable")
}

// TestUpdate_Happy exercises the Update path end-to-end: resolve
// instance ID, translate spec, call ApplyKargoInstance with only the
// Agents slice populated. No Get-before-Update — ApplyKargoInstance
// keys by Name inside the wire struct, eliminating the round-trip.
func TestUpdate_Happy(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")

	var capturedReq *kargov1.ApplyKargoInstanceRequest
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *kargov1.ApplyKargoInstanceRequest) error {
		capturedReq = req
		return nil
	}).Times(1)

	_, err := e.Update(context.Background(), a)
	require.NoError(t, err)
	require.NotNil(t, capturedReq)
	assert.Equal(t, "ki-1", capturedReq.GetId())
	require.Len(t, capturedReq.GetAgents(), 1)
}

// TestUpdate_ApplyErr covers the error path when ApplyKargoInstance
// fails.
func TestUpdate_ApplyErr(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).
		Return(errors.New("boom")).Times(1)
	_, err := e.Update(context.Background(), a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

// TestObserve_ProvisioningWait covers the short-circuit: the Kargo
// instance is still bootstrapping, so fetchAgent returns
// ProvisioningWait and Observe reports Unavailable + UpToDate to park
// the reconcile without escalating to ReconcileError.
func TestObserve_ProvisioningWait(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")
	mc.EXPECT().GetKargoInstanceAgent(gomock.Any(), "ki-1", "agt").
		Return(nil, provisioningWaitErr()).Times(1)

	obs, err := e.Observe(context.Background(), a)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.True(t, obs.ResourceUpToDate)
	got := a.Status.GetCondition(xpv1.TypeReady)
	assert.Equal(t, xpv1.Unavailable().Type, got.Type)
	assert.Equal(t, xpv1.Unavailable().Status, got.Status)
	assert.Equal(t, xpv1.Unavailable().Reason, got.Reason)
}

// TestObserve_GenericErrPropagates covers fetchAgent's non-transient
// error branch.
func TestObserve_GenericErrPropagates(t *testing.T) {
	e, mc := newExt(t)
	a := newAgent()
	meta.SetExternalName(a, "agt")
	mc.EXPECT().GetKargoInstanceAgent(gomock.Any(), "ki-1", "agt").
		Return(nil, errors.New("boom")).Times(1)
	_, err := e.Observe(context.Background(), a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
