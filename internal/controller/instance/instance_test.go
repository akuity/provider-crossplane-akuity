/*
Copyright 2022 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	mock_akuity_client "github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity/mock"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/test/fixtures"
)

var ctx = context.TODO()

func newExt(t *testing.T) (*external, *mock_akuity_client.MockClient) {
	t.Helper()
	mc := mock_akuity_client.NewMockClient(gomock.NewController(t))
	return &external{ExternalClient: base.ExternalClient{
		Client: mc,
		Logger: logging.NewNopLogger(),
	}}, mc
}

func TestCreate(t *testing.T) {
	applyInstanceRequest, err := BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)

	e, mc := newExt(t)

	mc.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(nil).Times(1)

	resp, err := e.Create(ctx, &fixtures.CrossplaneManagedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestCreate_ClientErr(t *testing.T) {
	applyInstanceRequest, err := BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)

	e, mc := newExt(t)

	mc.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(errors.New("fake")).Times(1)

	resp, err := e.Create(ctx, &fixtures.CrossplaneManagedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalCreation{}, resp)
}

func TestUpdate(t *testing.T) {
	applyInstanceRequest, err := BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)

	e, mc := newExt(t)

	mc.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(nil).Times(1)

	resp, err := e.Update(ctx, &fixtures.CrossplaneManagedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

func TestUpdate_ClientErr(t *testing.T) {
	applyInstanceRequest, err := BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)

	e, mc := newExt(t)

	mc.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(errors.New("fake")).Times(1)

	resp, err := e.Update(ctx, &fixtures.CrossplaneManagedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalUpdate{}, resp)
}

// TestUpdate_InvalidArgument_Terminal asserts that codes.InvalidArgument
// from ApplyInstance (e.g. argocdSecretRef populated with the reserved
// server.secretkey, or admin.password not in bcrypt format) is wrapped
// as reason.Terminal. Without classification, the next Observe re-fires
// Update from rotation drift and the gateway sees a steady ~15 ApplyInstance
// calls per minute against the same bad payload.
func TestUpdate_InvalidArgument_Terminal(t *testing.T) {
	applyInstanceRequest, err := BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)
	e, mc := newExt(t)
	mc.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(grpcstatus.Error(codes.InvalidArgument, "reserved key admin.password")).
		Times(1)
	_, err = e.Update(ctx, &fixtures.CrossplaneManagedInstance)
	require.Error(t, err)
	assert.True(t, reason.IsTerminal(err),
		"InvalidArgument from ApplyInstance must be reason.Terminal-classified, got %T %v", err, err)
}

// TestCreate_InvalidArgument_Terminal mirrors the Update assertion for
// Create also classifies a first-Apply rejection as terminal.
func TestCreate_InvalidArgument_Terminal(t *testing.T) {
	applyInstanceRequest, err := BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)
	e, mc := newExt(t)
	mc.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(grpcstatus.Error(codes.InvalidArgument, "admin.password not in bcrypt format")).
		Times(1)
	_, err = e.Create(ctx, &fixtures.CrossplaneManagedInstance)
	require.Error(t, err)
	assert.True(t, reason.IsTerminal(err),
		"InvalidArgument from ApplyInstance must be reason.Terminal-classified, got %T %v", err, err)
}

// TestUpdate_ProvisioningWait_NotTerminal locks in that the
// "still being provisioned" InvalidArgument variant stays retryable;
// it's a transient bootstrap signal, not a bad-input signal.
func TestUpdate_ProvisioningWait_NotTerminal(t *testing.T) {
	applyInstanceRequest, err := BuildApplyInstanceRequest(fixtures.CrossplaneManagedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)
	e, mc := newExt(t)
	mc.EXPECT().ApplyInstance(ctx, applyInstanceRequest).
		Return(grpcstatus.Error(codes.InvalidArgument, "instance still being provisioned")).
		Times(1)
	_, err = e.Update(ctx, &fixtures.CrossplaneManagedInstance)
	require.Error(t, err)
	assert.False(t, reason.IsTerminal(err))
	assert.True(t, reason.IsRetryable(err))
}

func TestDelete(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mc.EXPECT().DeleteInstance(ctx, fixtures.InstanceName).Return(nil).Times(1)

	resp, err := e.Delete(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalDelete{}, resp)
}

func TestDelete_EmptyExternalName(t *testing.T) {
	e, _ := newExt(t)
	resp, err := e.Delete(ctx, &v1alpha1.Instance{})
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalDelete{}, resp)
}

func TestDelete_ClearsTerminalWriteGuard(t *testing.T) {
	e, _ := newExt(t)
	e.TerminalWrites = base.NewTerminalWriteGuard()

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{Name: "bad-instance", Generation: 3}
	key, err := instanceTerminalWriteKey(&managedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)
	e.TerminalWrites.Record(key, reason.AsTerminal(errors.New("bad payload")))

	_, err = e.Delete(ctx, &managedInstance)
	require.NoError(t, err)

	_, _, ok := e.TerminalWrites.Suppress(&managedInstance, key)
	assert.False(t, ok)
}

func TestDelete_ClientErr(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mc.EXPECT().DeleteInstance(ctx, fixtures.InstanceName).Return(errors.New("fake")).Times(1)

	resp, err := e.Delete(ctx, &managedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalDelete{}, resp)
}

func TestObserve_EmptyExternalName(t *testing.T) {
	e, _ := newExt(t)
	resp, err := e.Observe(ctx, &v1alpha1.Instance{})
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_EmptyExternalName_SuppressesTerminalWrite(t *testing.T) {
	e, _ := newExt(t)
	e.TerminalWrites = base.NewTerminalWriteGuard()

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{Name: "bad-instance", Generation: 3}
	key, err := instanceTerminalWriteKey(&managedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)
	e.TerminalWrites.Record(key, reason.AsTerminal(errors.New("bad payload")))

	resp, err := e.Observe(ctx, &managedInstance)
	require.Error(t, err)
	assert.True(t, reason.IsTerminal(err))
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, resp)
	require.Len(t, managedInstance.Status.Conditions, 1)
	assert.Equal(t, xpv1.ReasonReconcileError, managedInstance.Status.Conditions[0].Reason)
}

// TestObserve_ExternalNameSet_SuppressesTerminalWriteBeforeGet covers
// the path where Crossplane's NameAsExternalName initializer has
// stamped the external-name annotation before the first reconcile.
// The terminal-write guard must short-circuit the Observe before any
// gateway round-trip, otherwise a Create that fails terminally on bad
// input loops Get->NotFound->Create on controller-runtime's exp-
// backoff. Asserts that GetInstance is NOT called when a matching
// terminal entry is cached.
func TestObserve_ExternalNameSet_SuppressesTerminalWriteBeforeGet(t *testing.T) {
	e, mc := newExt(t)
	e.TerminalWrites = base.NewTerminalWriteGuard()

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Name:       "bad-instance",
		Generation: 3,
		Annotations: map[string]string{
			"crossplane.io/external-name": "bad-instance",
		},
	}
	key, err := instanceTerminalWriteKey(&managedInstance, resolvedInstanceSecrets{})
	require.NoError(t, err)
	e.TerminalWrites.Record(key, reason.AsTerminal(errors.New("bad payload")))

	mc.EXPECT().GetInstance(gomock.Any(), gomock.Any()).Times(0)
	mc.EXPECT().ExportInstance(gomock.Any(), gomock.Any()).Times(0)

	resp, err := e.Observe(ctx, &managedInstance)
	require.Error(t, err)
	assert.True(t, reason.IsTerminal(err))
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, resp)
	require.Len(t, managedInstance.Status.Conditions, 1)
	assert.Equal(t, xpv1.ReasonReconcileError, managedInstance.Status.Conditions[0].Reason)
}

func TestDriftSpec_ArgocdConfigMapComparesOnlyUserKeys(t *testing.T) {
	presence := base.FieldPresence{
		Object:  true,
		Present: true,
		Children: map[string]base.FieldPresence{
			"argocdConfigMap": {
				Object:  true,
				Present: true,
				Children: map[string]base.FieldPresence{
					"exec.enabled": {Present: true},
				},
			},
		},
	}
	desired := v1alpha1.InstanceParameters{
		ArgoCDConfigMap: map[string]string{"exec.enabled": "true"},
	}
	observed := v1alpha1.InstanceParameters{
		ArgoCDConfigMap: map[string]string{
			"exec.enabled":                       "true",
			"application.resourceTrackingMethod": "annotation",
			"resource.respectRBAC":               "normal",
			"url":                                "https://platform.example",
		},
	}

	spec := driftSpec()
	spec.Presence = &presence
	ok, err := spec.UpToDate(ctx, &desired, &observed)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestNormalizeInstanceParameters_StripsIgnoredArgocdConfigMapKeysFromBothSides(t *testing.T) {
	desired := v1alpha1.InstanceParameters{
		ArgoCDConfigMap: map[string]string{
			"exec.enabled": "true",
			"url":          "https://user.example",
		},
	}
	observed := v1alpha1.InstanceParameters{
		ArgoCDConfigMap: map[string]string{
			"exec.enabled":         "true",
			"url":                  "https://platform.example",
			"resource.respectRBAC": "normal",
		},
	}

	normalizeInstanceParameters(&desired, &observed)

	assert.Equal(t, "true", desired.ArgoCDConfigMap["exec.enabled"])
	assert.Equal(t, "true", observed.ArgoCDConfigMap["exec.enabled"])
	for _, key := range ignoredArgocdCMKeys {
		assert.NotContains(t, desired.ArgoCDConfigMap, key)
		assert.NotContains(t, observed.ArgoCDConfigMap, key)
	}
}

func TestNormalizeInstanceParameters_DisabledBucketRateLimitingAdoptsServerScalars(t *testing.T) {
	desired := v1alpha1.InstanceParameters{
		ArgoCD: &crossplanetypes.ArgoCD{
			Spec: crossplanetypes.ArgoCDSpec{
				InstanceSpec: crossplanetypes.InstanceSpec{
					AppReconciliationsRateLimiting: &crossplanetypes.AppReconciliationsRateLimiting{
						BucketRateLimiting: &crossplanetypes.BucketRateLimiting{
							BucketSize: 200,
							BucketQps:  50,
						},
					},
				},
			},
		},
	}
	observed := v1alpha1.InstanceParameters{
		ArgoCD: &crossplanetypes.ArgoCD{
			Spec: crossplanetypes.ArgoCDSpec{
				InstanceSpec: crossplanetypes.InstanceSpec{
					AppReconciliationsRateLimiting: &crossplanetypes.AppReconciliationsRateLimiting{
						BucketRateLimiting: &crossplanetypes.BucketRateLimiting{
							Enabled:    ptr.To(false),
							BucketSize: 500,
							BucketQps:  50,
						},
					},
				},
			},
		},
	}

	normalizeInstanceParameters(&desired, &observed)

	bucket := desired.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting.BucketRateLimiting
	require.NotNil(t, bucket.Enabled)
	assert.False(t, *bucket.Enabled)
	assert.Equal(t, uint32(500), bucket.BucketSize)
	assert.Equal(t, uint32(50), bucket.BucketQps)
}

func TestNormalizeInstanceParameters_EnabledBucketRateLimitingKeepsUserScalars(t *testing.T) {
	desired := v1alpha1.InstanceParameters{
		ArgoCD: &crossplanetypes.ArgoCD{
			Spec: crossplanetypes.ArgoCDSpec{
				InstanceSpec: crossplanetypes.InstanceSpec{
					AppReconciliationsRateLimiting: &crossplanetypes.AppReconciliationsRateLimiting{
						BucketRateLimiting: &crossplanetypes.BucketRateLimiting{
							Enabled:    ptr.To(true),
							BucketSize: 200,
							BucketQps:  50,
						},
					},
				},
			},
		},
	}
	observed := v1alpha1.InstanceParameters{
		ArgoCD: &crossplanetypes.ArgoCD{
			Spec: crossplanetypes.ArgoCDSpec{
				InstanceSpec: crossplanetypes.InstanceSpec{
					AppReconciliationsRateLimiting: &crossplanetypes.AppReconciliationsRateLimiting{
						BucketRateLimiting: &crossplanetypes.BucketRateLimiting{
							Enabled:    ptr.To(true),
							BucketSize: 500,
							BucketQps:  50,
						},
					},
				},
			},
		},
	}

	normalizeInstanceParameters(&desired, &observed)

	bucket := desired.ArgoCD.Spec.InstanceSpec.AppReconciliationsRateLimiting.BucketRateLimiting
	assert.Equal(t, uint32(200), bucket.BucketSize)
	assert.Equal(t, uint32(50), bucket.BucketQps)
}

func TestObserve_GetInstanceNotFoundErr(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(nil, reason.AsNotFound(errors.New("not found"))).Times(1)

	resp, err := e.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, resp)
}

func TestObserve_GetInstanceErr(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(nil, errors.New("fake")).Times(1)

	resp, err := e.Observe(ctx, &managedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
	assert.Equal(t, xpv1.ReasonReconcileError, managedInstance.Status.Conditions[0].Reason)
}

func TestObserve_ExportInstanceErr(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mc.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(nil, errors.New("fake")).Times(1)

	resp, err := e.Observe(ctx, &managedInstance)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{}, resp)
	assert.Equal(t, xpv1.ReasonReconcileError, managedInstance.Status.Conditions[0].Reason)
}

func TestObserve_HealthStatusNotHealthy(t *testing.T) {
	e, mc := newExt(t)

	akuityInstance := fixtures.AkuityInstance
	akuityInstance.HealthStatus = &health.Status{
		Code:    health.StatusCode_STATUS_CODE_DEGRADED,
		Message: "Instance is degraded",
	}

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mc.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	_, err := e.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, xpv1.Unavailable().Reason, managedInstance.Status.Conditions[0].Reason)
}

func TestObserve_HealthStatusHealthy(t *testing.T) {
	e, mc := newExt(t)

	akuityInstance := fixtures.AkuityInstance
	akuityInstance.HealthStatus = &health.Status{
		Code:    health.StatusCode_STATUS_CODE_HEALTHY,
		Message: "Instance is healthy",
	}

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mc.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	_, err := e.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, xpv1.Available().Reason, managedInstance.Status.Conditions[0].Reason)
}

func TestObserve_InstanceUpToDate(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mc.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	resp, err := e.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, resp)
}

func TestObserve_InstanceNotUpToDate(t *testing.T) {
	e, mc := newExt(t)

	managedInstance := fixtures.CrossplaneManagedInstance
	managedInstance.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"crossplane.io/external-name": fixtures.InstanceName,
		},
	}
	managedInstance.Spec.ForProvider.ArgoCD.Spec.Description = "new-description"

	mc.EXPECT().GetInstance(ctx, fixtures.InstanceName).
		Return(fixtures.AkuityInstance, nil).Times(1)

	mc.EXPECT().ExportInstance(ctx, fixtures.InstanceName).
		Return(&argocdv1.ExportInstanceResponse{}, nil).Times(1)

	resp, err := e.Observe(ctx, &managedInstance)
	require.NoError(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, resp)
}
