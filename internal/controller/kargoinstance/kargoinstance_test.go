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
	"time"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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
	return grpcstatus.Error(codes.InvalidArgument, "instance still being provisioned")
}

func newKI() *v1alpha1.KargoInstance {
	return &v1alpha1.KargoInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "ki", Namespace: "ns"},
		Spec: v1alpha1.KargoInstanceSpec{
			ForProvider: v1alpha1.KargoInstanceParameters{
				Name:  "ki",
				Kargo: crossplanetypes.KargoSpec{Version: "v1.0.0"},
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

func mustCMStruct(t *testing.T, data map[string]string) *structpb.Struct {
	t.Helper()
	cmData := make(map[string]interface{}, len(data))
	for k, v := range data {
		cmData[k] = v
	}
	pb, err := structpb.NewStruct(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]interface{}{"name": "kargo-cm"},
		"data":       cmData,
	})
	require.NoError(t, err)
	return pb
}

// TestKargoConfigMapUpToDate_SubsetBehavior exercises the 7.A drift
// check: desired ⊆ observed keeps ResourceUpToDate=true; any missing
// or divergent key drops it to false so the next Apply can self-heal.
func TestKargoConfigMapUpToDate_SubsetBehavior(t *testing.T) {
	desired := map[string]string{"foo": "bar", "baz": "qux"}

	// Exact match.
	exp := &kargov1.ExportKargoInstanceResponse{
		KargoConfigmap: mustCMStruct(t, map[string]string{"foo": "bar", "baz": "qux"}),
	}
	ok, _, err := kargoConfigMapUpToDate(desired, exp)
	require.NoError(t, err)
	assert.True(t, ok)

	// Observed has extra key — still up to date under subset semantics.
	exp = &kargov1.ExportKargoInstanceResponse{
		KargoConfigmap: mustCMStruct(t, map[string]string{"foo": "bar", "baz": "qux", "extra": "x"}),
	}
	ok, _, err = kargoConfigMapUpToDate(desired, exp)
	require.NoError(t, err)
	assert.True(t, ok, "extra server-side keys must not fire drift")

	// Missing key — drift.
	exp = &kargov1.ExportKargoInstanceResponse{
		KargoConfigmap: mustCMStruct(t, map[string]string{"foo": "bar"}),
	}
	ok, observed, err := kargoConfigMapUpToDate(desired, exp)
	require.NoError(t, err)
	assert.False(t, ok, "missing desired key must fire drift")
	assert.Equal(t, "bar", observed["foo"])

	// Divergent value — drift.
	exp = &kargov1.ExportKargoInstanceResponse{
		KargoConfigmap: mustCMStruct(t, map[string]string{"foo": "wrong", "baz": "qux"}),
	}
	ok, _, err = kargoConfigMapUpToDate(desired, exp)
	require.NoError(t, err)
	assert.False(t, ok, "divergent value must fire drift")

	// Server returned no ConfigMap struct — treat as empty, all
	// desired keys are missing → drift.
	exp = &kargov1.ExportKargoInstanceResponse{}
	ok, _, err = kargoConfigMapUpToDate(desired, exp)
	require.NoError(t, err)
	assert.False(t, ok, "absent gateway ConfigMap must fire drift")

	// Empty desired — nothing to compare.
	ok, _, err = kargoConfigMapUpToDate(nil, exp)
	require.NoError(t, err)
	assert.True(t, ok)
}

// TestObserve_RepoCredsTTL_ForcesReapply covers H3: past the TTL on
// spec.forProvider.kargoRepoCredentialSecretRefs the controller must
// return ResourceUpToDate=false even when nothing else has changed,
// so a server-side OOB deletion of the credential gets re-Applied.
func TestObserve_RepoCredsTTL_ForcesReapply(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	backing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "k8s-secret"},
		Data:       map[string][]byte{"password": []byte("p")},
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backing).Build()

	mc := mockclient.NewMockClient(gomock.NewController(t))
	e := &external{ExternalClient: base.ExternalClient{Client: mc, Kube: kube, Logger: logging.NewNopLogger()}}

	ki := newKI()
	ki.Spec.ForProvider.KargoRepoCredentialSecretRefs = []v1alpha1.KargoRepoCredentialSecretRef{{
		Name:             "repo-github",
		ProjectNamespace: "platform",
		CredType:         "git",
		SecretRef:        xpv1.LocalSecretReference{Name: "k8s-secret"},
	}}
	meta.SetExternalName(ki, "ki")

	// Bootstrap: simulate the hash already being up to date so the
	// only thing that could drop upToDate is the TTL check.
	sec, err := resolveKargoSecrets(context.Background(), kube, ki)
	require.NoError(t, err)
	ki.Status.AtProvider.SecretHash = sec.Hash()

	mc.EXPECT().GetKargoInstance(gomock.Any(), "ki").Return(&kargov1.KargoInstance{
		Id: "id-1", Name: "ki", Version: "v1.0.0",
		HealthStatus: &health.Status{Code: health.StatusCode_STATUS_CODE_HEALTHY},
	}, nil).Times(2)

	// First Observe: RepoCredsAppliedAt is nil → treat as "past TTL"
	// so the very first reconcile schedules an Apply.
	obs, err := e.Observe(context.Background(), ki)
	require.NoError(t, err)
	assert.False(t, obs.ResourceUpToDate, "nil RepoCredsAppliedAt must force re-Apply to bootstrap freshness tracking")

	// Second Observe with a recent RepoCredsAppliedAt must report up
	// to date again — the TTL is what prevents stampede.
	recent := metav1.NewTime(time.Now().Add(-5 * time.Minute))
	ki.Status.AtProvider.RepoCredsAppliedAt = &recent
	obs, err = e.Observe(context.Background(), ki)
	require.NoError(t, err)
	assert.True(t, obs.ResourceUpToDate, "within-TTL timestamp must suppress forced re-Apply")
}

func TestExtractKargoConfigMapData_ShapeGuards(t *testing.T) {
	got, err := extractKargoConfigMapData(nil)
	require.NoError(t, err)
	assert.Nil(t, got)

	pb, err := structpb.NewStruct(map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
	})
	require.NoError(t, err)
	got, err = extractKargoConfigMapData(pb)
	require.NoError(t, err)
	assert.Nil(t, got, "struct without data key is treated as empty")

	pb, err = structpb.NewStruct(map[string]interface{}{
		"data": "not-a-map",
	})
	require.NoError(t, err)
	_, err = extractKargoConfigMapData(pb)
	require.Error(t, err, "non-object data must surface an error so drift doesn't silently pass")
}

// TestUpdate_DelegatesToApply covers the Update path: Update must reuse
// apply() so the same orchestration (secrets, configmap, spec,
// repo-creds) runs once the external name is set.
func TestUpdate_DelegatesToApply(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	_, err := e.Update(context.Background(), ki)
	require.NoError(t, err)
}

// TestUpdate_ApplyErr surfaces gateway errors on Update.
func TestUpdate_ApplyErr(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).
		Return(errors.New("boom")).Times(1)
	_, err := e.Update(context.Background(), ki)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

// TestCreate_ApplyErr covers the Create error path — currently only
// the happy path is tested; a regression here would silently accept
// ApplyKargoInstance failures.
func TestCreate_ApplyErr(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).
		Return(errors.New("boom")).Times(1)
	_, err := e.Create(context.Background(), ki)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	assert.Empty(t, meta.GetExternalName(ki), "external name must not be set on Apply failure")
}

// TestUpdate_InvalidArgument_Terminal asserts that the gateway returning
// codes.InvalidArgument from Apply (e.g. a SecretRef populated with a
// reserved key like server.secretkey or a Kargo repo cred whose project
// namespace doesn't yet exist on the cluster) is wrapped as
// reason.Terminal so retries don't hammer portal-server until the user
// fixes the spec.
func TestUpdate_InvalidArgument_Terminal(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).
		Return(grpcstatus.Error(codes.InvalidArgument, "reserved key admin.password")).
		Times(1)
	_, err := e.Update(context.Background(), ki)
	require.Error(t, err)
	assert.True(t, reason.IsTerminal(err),
		"InvalidArgument from ApplyKargoInstance must be reason.Terminal-classified, got %T %v", err, err)
}

// TestCreate_InvalidArgument_Terminal mirrors the Update assertion for
// the Create path — a first-Apply rejection must not hot-loop either.
func TestCreate_InvalidArgument_Terminal(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).
		Return(grpcstatus.Error(codes.InvalidArgument, "kargo project namespace not found")).
		Times(1)
	_, err := e.Create(context.Background(), ki)
	require.Error(t, err)
	assert.True(t, reason.IsTerminal(err),
		"InvalidArgument from ApplyKargoInstance must be reason.Terminal-classified, got %T %v", err, err)
	assert.Empty(t, meta.GetExternalName(ki))
}

// TestUpdate_ProvisioningWait_NotTerminal locks in that the
// "still being provisioned" InvalidArgument substring stays retryable
// rather than being downgraded to Terminal — KargoInstance bootstrapping
// is transient.
func TestUpdate_ProvisioningWait_NotTerminal(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().ApplyKargoInstance(gomock.Any(), gomock.Any()).
		Return(provisioningWaitErr()).Times(1)
	_, err := e.Update(context.Background(), ki)
	require.Error(t, err)
	assert.False(t, reason.IsTerminal(err),
		"provisioning-wait InvalidArgument must stay retryable, got Terminal: %v", err)
	assert.True(t, reason.IsRetryable(err))
}

// TestDelete_EmptyExternalName short-circuits before the gateway call
// so Crossplane can release the finalizer on MRs that never got an
// external name (Create failed before SetExternalName).
func TestDelete_EmptyExternalName(t *testing.T) {
	e, _ := newExt(t)
	ki := newKI()
	// external-name deliberately unset.
	_, err := e.Delete(context.Background(), ki)
	require.NoError(t, err)
}

// TestDelete_GenericErrPropagates surfaces gateway errors on Delete.
func TestDelete_GenericErrPropagates(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().DeleteKargoInstance(gomock.Any(), "ki").
		Return(errors.New("boom")).Times(1)
	_, err := e.Delete(context.Background(), ki)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

// TestObserve_ProvisioningWait covers the short-circuit contract: the
// gateway reports the KargoInstance is still bootstrapping, so the
// controller parks it Unavailable + UpToDate to stop re-applying while
// Crossplane waits.
func TestObserve_ProvisioningWait(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().GetKargoInstance(gomock.Any(), "ki").
		Return(nil, provisioningWaitErr()).Times(1)

	obs, err := e.Observe(context.Background(), ki)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.True(t, obs.ResourceUpToDate)
	got := ki.Status.GetCondition(xpv1.TypeReady)
	assert.Equal(t, xpv1.Unavailable().Type, got.Type)
	assert.Equal(t, xpv1.Unavailable().Status, got.Status)
	assert.Equal(t, xpv1.Unavailable().Reason, got.Reason)
}

// TestObserve_GenericErrPropagates covers Get's non-transient error
// branch — surface rather than swallow.
func TestObserve_GenericErrPropagates(t *testing.T) {
	e, mc := newExt(t)
	ki := newKI()
	meta.SetExternalName(ki, "ki")
	mc.EXPECT().GetKargoInstance(gomock.Any(), "ki").
		Return(nil, errors.New("boom")).Times(1)
	_, err := e.Observe(context.Background(), ki)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
