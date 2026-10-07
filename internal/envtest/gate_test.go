//go:build envtest

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

package envtest_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/gate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/customresourcesgate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/akuityio/provider-crossplane-akuity/apis"
	v1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	akuity "github.com/akuityio/provider-crossplane-akuity/internal/controller"
)

// Controllers start only once their CRD is established.
func TestSetupGated_StartsOnlyInstalledKinds(t *testing.T) {
	crds := filepath.Join("..", "..", "package", "crds")
	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(crds, "akuity.crossplane.io_providerconfigs.yaml"),
			filepath.Join(crds, "akuity.crossplane.io_providerconfigusages.yaml"),
			filepath.Join(crds, "core.akuity.crossplane.io_instances.yaml"),
		},
		ErrorIfCRDPathMissing: true,
	}
	restCfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.Stop() })

	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, apis.AddToScheme(s))
	require.NoError(t, apiextensionsv1.AddToScheme(s))

	const cacheSyncTimeout = 3 * time.Second
	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme:     s,
		Metrics:    metricsserver.Options{BindAddress: "0"},
		Controller: config.Controller{CacheSyncTimeout: cacheSyncTimeout},
	})
	require.NoError(t, err)

	o := controller.Options{
		Logger:                  logging.NewNopLogger(),
		MaxConcurrentReconciles: 1,
		PollInterval:            time.Minute,
		GlobalRateLimiter:       ratelimiter.NewGlobal(10),
		Features:                &feature.Flags{},
		Gate:                    new(gate.Gate[schema.GroupVersionKind]),
		Groups:                  akuity.Groups,
	}
	require.NoError(t, customresourcesgate.Setup(mgr, o))
	require.NoError(t, akuity.SetupGated(mgr, o))

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- mgr.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-errCh)
	})

	c, err := client.New(restCfg, client.Options{Scheme: s})
	require.NoError(t, err)

	inst := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "gated-instance"},
		Spec: v1alpha1.InstanceSpec{
			ForProvider: v1alpha1.InstanceParameters{Name: "gated-instance", ArgoCD: minimalArgoCD()},
		},
	}
	require.NoError(t, c.Create(ctx, inst))
	requireReconciled(ctx, t, c, inst)

	require.Never(t, func() bool { return len(errCh) > 0 }, 2*cacheSyncTimeout, 200*time.Millisecond,
		"manager must not exit while inactive kinds have no CRD")

	_, err = envtest.InstallCRDs(restCfg, envtest.CRDInstallOptions{
		Paths:              []string{filepath.Join(crds, "core.akuity.crossplane.io_kargoagents.yaml")},
		ErrorIfPathMissing: true,
	})
	require.NoError(t, err)

	ka := &v1alpha1.KargoAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "gated-kargoagent"},
		Spec: v1alpha1.KargoAgentSpec{
			ForProvider: v1alpha1.KargoAgentParameters{KargoInstanceID: "ki-gated", Name: "gated-kargoagent"},
		},
	}
	require.NoError(t, c.Create(ctx, ka))
	requireReconciled(ctx, t, c, ka)
}

// requireReconciled waits until mg reports a Synced condition.
func requireReconciled(ctx context.Context, t *testing.T, c client.Client, mg resource.Managed) {
	t.Helper()
	require.Eventually(t, func() bool {
		if err := c.Get(ctx, client.ObjectKeyFromObject(mg), mg); err != nil {
			return false
		}
		return mg.GetCondition(xpv2.TypeSynced).Status != corev1.ConditionUnknown
	}, 30*time.Second, 200*time.Millisecond, "%T was not reconciled", mg)
}
