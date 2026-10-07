/*
Copyright 2020 The Crossplane Authors.

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

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/pkg/errors"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/gate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/customresourcesgate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"

	"github.com/akuityio/provider-crossplane-akuity/apis"
	akuity "github.com/akuityio/provider-crossplane-akuity/internal/controller"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/ratelimit"
)

func main() {
	var (
		app            = kingpin.New(filepath.Base(os.Args[0]), "Akuity support for Crossplane.").DefaultEnvars()
		debug          = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		leaderElection = app.Flag("leader-election", "Use leader election for the controller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").Bool()

		syncInterval            = app.Flag("sync", "How often all resources will be double-checked for drift from the desired state.").Short('s').Default("1h").Duration()
		pollInterval            = app.Flag("poll", "How often individual resources will be checked for drift from the desired state").Default("1m").Duration()
		pollStateMetricInterval = app.Flag("poll-state-metric", "State metric recording interval").Default("5s").Duration()
		maxReconcileRate        = app.Flag("max-reconcile-rate", "The global maximum rate per second at which resources may checked for drift from the desired state.").Default("10").Int()

		enableSecretCache = app.Flag("enable-secret-cache", "Enable caching of Secret objects. When true, Secrets are served from the informer cache instead of direct API calls. This reduces API server load but increases memory usage.").Default("true").Envar("ENABLE_SECRET_CACHE").Bool()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("provider-crossplane-akuity"))
	if *debug {
		// controller-runtime is noisy even at info level, so wire its
		// logger only when debug logging is requested.
		ctrl.SetLogger(zl)
	} else {
		// controller-runtime warns when no logger was ever set; hand it
		// a discarding one so the warning does not land in the pod logs.
		ctrl.SetLogger(zap.New(zap.WriteTo(io.Discard)))
	}

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	// Secrets are read on every Connect (provider credentials) and on
	// every connection-detail publish. Serving them from the informer
	// cache avoids API calls at the cost of caching every Secret in the
	// cluster; operators can opt out on large clusters.
	var clientOpts client.Options
	if !*enableSecretCache {
		clientOpts = client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{&corev1.Secret{}},
			},
		}
	}

	scheme, err := newScheme()
	kingpin.FatalIfError(err, "Cannot build scheme")

	mgr, err := ctrl.NewManager(ratelimiter.LimitRESTConfig(cfg, *maxReconcileRate), ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			SyncPeriod: syncInterval,
			// The CRD gate only needs each CRD's group, kind, served
			// versions and Established condition. Strip the OpenAPI
			// schema and managed fields before CRDs enter the cache so
			// a cluster full of large CRDs does not inflate memory.
			// Never write a cached CRD back with a full Update: it
			// would persist the stripped object.
			ByObject: map[client.Object]cache.ByObject{
				&apiextensionsv1.CustomResourceDefinition{}: {
					Transform: customresourcesgate.TransformStripCRDSchema,
				},
			},
		},
		Client: clientOpts,

		// controller-runtime defaults to ConfigMaps+Leases with short
		// lease timings. Under high reconcile load, API-server pressure
		// can exceed the renewal deadline; use Leases only with longer
		// timings to avoid unnecessary leader loss.
		LeaderElection:                *leaderElection,
		LeaderElectionID:              "crossplane-leader-election-provider-crossplane-akuity",
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 new(60 * time.Second),
		RenewDeadline:                 new(50 * time.Second),
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")

	metricRecorder := managed.NewMRMetricRecorder()
	stateMetrics := statemetrics.NewMRStateMetrics()
	metrics.Registry.MustRegister(metricRecorder, stateMetrics)

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: *maxReconcileRate,
		PollInterval:            *pollInterval,
		GlobalRateLimiter:       ratelimit.ForAkuity(*maxReconcileRate),
		Features:                &feature.Flags{},
		// Limit the CRD gate to this provider's API groups so unrelated
		// CRD churn in the cluster does not trigger gate reconciles.
		Groups: akuity.Groups,
		MetricOptions: &controller.MetricOptions{
			PollStateMetricInterval: *pollStateMetricInterval,
			MRMetrics:               metricRecorder,
			MRStateMetrics:          stateMetrics,
		},
	}

	canSafeStart, err := canWatchCRD(context.Background(), mgr)
	kingpin.FatalIfError(err, "SafeStart precheck failed")
	if canSafeStart {
		o.Gate = new(gate.Gate[schema.GroupVersionKind])
		kingpin.FatalIfError(customresourcesgate.Setup(mgr, o), "Cannot setup CRD gate")
		kingpin.FatalIfError(akuity.SetupGated(mgr, o), "Cannot setup Akuity controllers")
	} else {
		log.Info("Provider has missing RBAC permissions for watching CRDs, controller SafeStart capability will be disabled")
		kingpin.FatalIfError(akuity.Setup(mgr, o), "Cannot setup Akuity controllers")
	}
	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}

// newScheme registers the provider's own APIs, CustomResourceDefinitions
// for the SafeStart gate, and only the client-go kinds the manager client
// talks to: Secrets for credentials and connection details, and
// SelfSubjectAccessReview for the SafeStart RBAC precheck. Every group
// version also gets the meta types that list, watch and create calls need.
func newScheme() (*runtime.Scheme, error) {
	s := runtime.NewScheme()
	if err := apis.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add Akuity APIs to scheme")
	}
	if err := apiextensionsv1.AddToScheme(s); err != nil {
		return nil, errors.Wrap(err, "cannot add api-extensions APIs to scheme")
	}
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{}, &corev1.SecretList{})
	metav1.AddToGroupVersion(s, corev1.SchemeGroupVersion)
	s.AddKnownTypes(authv1.SchemeGroupVersion, &authv1.SelfSubjectAccessReview{})
	metav1.AddToGroupVersion(s, authv1.SchemeGroupVersion)
	return s, nil
}

// canWatchCRD reports whether the provider can get, list and watch CRDs.
func canWatchCRD(ctx context.Context, mgr ctrl.Manager) (bool, error) {
	for _, verb := range []string{"get", "list", "watch"} {
		sar := &authv1.SelfSubjectAccessReview{
			Spec: authv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authv1.ResourceAttributes{
					Group:    apiextensionsv1.GroupName,
					Resource: "customresourcedefinitions",
					Verb:     verb,
				},
			},
		}
		if err := mgr.GetClient().Create(ctx, sar); err != nil {
			return false, errors.Wrapf(err, "cannot check %s permission on CustomResourceDefinitions", verb)
		}
		if !sar.Status.Allowed {
			return false, nil
		}
	}
	return true, nil
}
