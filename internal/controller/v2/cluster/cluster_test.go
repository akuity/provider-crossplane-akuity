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

package cluster

import (
	"context"
	"testing"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	health "github.com/akuity/api-client-go/pkg/api/gen/types/status/health/v1"
	reconv1 "github.com/akuity/api-client-go/pkg/api/gen/types/status/reconciliation/v1"
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

func newCluster() *v1alpha2.Cluster {
	return &v1alpha2.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
		Spec: v1alpha2.ClusterSpec{
			ForProvider: v1alpha2.ClusterParameters{
				InstanceID:  "inst-1",
				Name:        "c",
				Description: "test cluster",
				Data: v1alpha2.ClusterData{
					Size: v1alpha2.ClusterSize("small"),
				},
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
	obs, err := e.Observe(context.Background(), newCluster())
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_NotFound(t *testing.T) {
	e, mc := newExt(t)
	cl := newCluster()
	meta.SetExternalName(cl, "c")

	mc.EXPECT().GetCluster(gomock.Any(), "inst-1", "c").
		Return(nil, reason.AsNotFound(errors.New("nope"))).Times(1)

	obs, err := e.Observe(context.Background(), cl)
	require.NoError(t, err)
	assert.False(t, obs.ResourceExists)
}

func TestObserve_NotYetReconciled(t *testing.T) {
	e, mc := newExt(t)
	cl := newCluster()
	meta.SetExternalName(cl, "c")

	ac := &argocdv1.Cluster{
		Id:                   "cid-1",
		Name:                 "c",
		ReconciliationStatus: &reconv1.Status{Code: reconv1.StatusCode_STATUS_CODE_PROGRESSING},
		HealthStatus:         &health.Status{Code: health.StatusCode_STATUS_CODE_PROGRESSING},
	}
	mc.EXPECT().GetCluster(gomock.Any(), "inst-1", "c").Return(ac, nil).Times(1)
	// No GetClusterManifestsOnce call expected — reconcile not yet terminal.

	obs, err := e.Observe(context.Background(), cl)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.False(t, obs.ResourceUpToDate)
	assert.Nil(t, obs.ConnectionDetails)
}

func TestObserve_ReconciledNoKubeConfigSource(t *testing.T) {
	e, mc := newExt(t)
	cl := newCluster()
	meta.SetExternalName(cl, "c")

	ac := &argocdv1.Cluster{
		Id:                   "cid-1",
		Name:                 "c",
		Description:          "test cluster",
		Data:                 &argocdv1.ClusterData{Size: argocdv1.ClusterSize_CLUSTER_SIZE_SMALL},
		ReconciliationStatus: &reconv1.Status{Code: reconv1.StatusCode_STATUS_CODE_SUCCESSFUL},
		HealthStatus:         &health.Status{Code: health.StatusCode_STATUS_CODE_HEALTHY},
	}
	mc.EXPECT().GetCluster(gomock.Any(), "inst-1", "c").Return(ac, nil).Times(1)
	// No GetClusterManifestsOnce — user did not opt into inline apply.

	obs, err := e.Observe(context.Background(), cl)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.True(t, obs.ResourceUpToDate)
	assert.Nil(t, obs.ConnectionDetails)
	assert.Equal(t, "cid-1", cl.Status.AtProvider.ID)
	assert.Equal(t, int32(health.StatusCode_STATUS_CODE_HEALTHY), cl.Status.AtProvider.HealthStatus.Code)
}

func TestObserve_InlineKubeConfigDriftsWhenHashMissing(t *testing.T) {
	e, mc := newExt(t)
	cl := newCluster()
	meta.SetExternalName(cl, "c")
	cl.Spec.ForProvider.EnableInClusterKubeConfig = true

	ac := &argocdv1.Cluster{
		Id:                   "cid-1",
		Name:                 "c",
		Description:          "test cluster",
		Data:                 &argocdv1.ClusterData{Size: argocdv1.ClusterSize_CLUSTER_SIZE_SMALL},
		ReconciliationStatus: &reconv1.Status{Code: reconv1.StatusCode_STATUS_CODE_SUCCESSFUL},
		HealthStatus:         &health.Status{Code: health.StatusCode_STATUS_CODE_HEALTHY},
	}
	mc.EXPECT().GetCluster(gomock.Any(), "inst-1", "c").Return(ac, nil).Times(1)
	mc.EXPECT().GetClusterManifestsOnce(gomock.Any(), "inst-1", "cid-1").
		Return("apiVersion: v1\nkind: ConfigMap\n", nil).Times(1)

	obs, err := e.Observe(context.Background(), cl)
	require.NoError(t, err)
	assert.True(t, obs.ResourceExists)
	assert.False(t, obs.ResourceUpToDate)
	assert.Equal(t, "cid-1", cl.Status.AtProvider.ID)
}

func TestObserve_InlineKubeConfigUpToDateWhenHashMatches(t *testing.T) {
	e, mc := newExt(t)
	cl := newCluster()
	meta.SetExternalName(cl, "c")
	cl.Spec.ForProvider.EnableInClusterKubeConfig = true

	manifests := "apiVersion: v1\nkind: ConfigMap\n"
	cl.Status.AtProvider.AgentManifestsHash = hashManifests(manifests)

	ac := &argocdv1.Cluster{
		Id:                   "cid-1",
		Name:                 "c",
		Description:          "test cluster",
		Data:                 &argocdv1.ClusterData{Size: argocdv1.ClusterSize_CLUSTER_SIZE_SMALL},
		ReconciliationStatus: &reconv1.Status{Code: reconv1.StatusCode_STATUS_CODE_SUCCESSFUL},
		HealthStatus:         &health.Status{Code: health.StatusCode_STATUS_CODE_HEALTHY},
	}
	mc.EXPECT().GetCluster(gomock.Any(), "inst-1", "c").Return(ac, nil).Times(1)
	mc.EXPECT().GetClusterManifestsOnce(gomock.Any(), "inst-1", "cid-1").Return(manifests, nil).Times(1)

	obs, err := e.Observe(context.Background(), cl)
	require.NoError(t, err)
	assert.True(t, obs.ResourceUpToDate)
}

func TestCreate_CallsApply(t *testing.T) {
	e, mc := newExt(t)
	cl := newCluster()

	mc.EXPECT().ApplyCluster(gomock.Any(), "inst-1", gomock.Any()).Return(nil).Times(1)

	_, err := e.Create(context.Background(), cl)
	require.NoError(t, err)
	assert.Equal(t, "c", meta.GetExternalName(cl))
}

func TestUpdate_CallsApply(t *testing.T) {
	e, mc := newExt(t)
	cl := newCluster()

	mc.EXPECT().ApplyCluster(gomock.Any(), "inst-1", gomock.Any()).Return(nil).Times(1)

	_, err := e.Update(context.Background(), cl)
	require.NoError(t, err)
}

func TestDelete_NoExternalName(t *testing.T) {
	e, _ := newExt(t)
	_, err := e.Delete(context.Background(), newCluster())
	require.NoError(t, err)
}

func TestDelete_CallsDeleteCluster(t *testing.T) {
	e, mc := newExt(t)
	cl := newCluster()
	meta.SetExternalName(cl, "c")

	mc.EXPECT().DeleteCluster(gomock.Any(), "inst-1", "c").Return(nil).Times(1)

	_, err := e.Delete(context.Background(), cl)
	require.NoError(t, err)
}

// TestDelete_RemoveAgentNoKubeConfigSourceSkipsTargetCleanup verifies
// the controller-side defensive check (belt-and-suspenders alongside
// the CEL rule): even if RemoveAgentResourcesOnDestroy is true, when
// neither kubeconfig field is set we skip the target-cluster teardown
// entirely and only call the platform DeleteCluster. No GetCluster or
// GetClusterManifestsOnce expectation means they must not be called.
func TestDelete_RemoveAgentNoKubeConfigSourceSkipsTargetCleanup(t *testing.T) {
	e, mc := newExt(t)
	cl := newCluster()
	meta.SetExternalName(cl, "c")
	cl.Spec.ForProvider.RemoveAgentResourcesOnDestroy = true

	mc.EXPECT().DeleteCluster(gomock.Any(), "inst-1", "c").Return(nil).Times(1)

	_, err := e.Delete(context.Background(), cl)
	require.NoError(t, err)
}

func TestResolveInstanceID_MissingRefAndID(t *testing.T) {
	e, _ := newExt(t)
	cl := &v1alpha2.Cluster{Spec: v1alpha2.ClusterSpec{ForProvider: v1alpha2.ClusterParameters{Name: "c"}}}
	_, err := e.resolveInstanceID(context.Background(), cl)
	require.Error(t, err)
}
