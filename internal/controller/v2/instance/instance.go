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

// Package instance is the v1alpha2 Instance controller. It mirrors the
// legacy v1alpha1 flow but consumes the curated v1alpha2.Instance shape
// via the codegen-emitted converters in internal/convert and the
// generic BaseConnector from internal/controller/base.
package instance

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/go-cmp/cmp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/event"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	apisv1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// Setup registers the v1alpha2 Instance controller with the supplied
// manager. The controller uses the modern (typed) ProviderConfigUsage
// tracker, reconciles on ResourceSpec changes only via
// DesiredStateChanged, and routes rate limiting through the shared
// Akuity workqueue limiter.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha2.InstanceGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewRecorder(mgr, name)

	conn := &base.Connector[*v1alpha2.Instance]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha2.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha2.Instance] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.InstanceGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha2.Instance](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.Instance{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// external implements managed.TypedExternalClient[*v1alpha2.Instance].
type external struct {
	base.ExternalClient
}

// setSecretHash stores the SHA256 of every resolved Secret on the
// managed resource's status. managed.Reconciler persists AtProvider
// after Create/Update, so this value survives the reconcile round-trip
// and remains available for the next Observe to detect rotation.
// Storing it in status rather than an annotation avoids an extra kube
// client Patch call and keeps the lifecycle under the reconciler's
// control.
func setSecretHash(mg *v1alpha2.Instance, h string) {
	mg.Status.AtProvider.SecretHash = h
}

func getSecretHash(mg *v1alpha2.Instance) string {
	return mg.Status.AtProvider.SecretHash
}

//nolint:gocyclo // Observe coordinates struct cmp + export-based child drift + secret hash; the linear branching is the simplest readable form.
func (e *external) Observe(ctx context.Context, mg *v1alpha2.Instance) (managed.ExternalObservation, error) {
	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	name := meta.GetExternalName(mg)
	ai, err := e.Client.GetInstance(ctx, name)
	if err != nil {
		if reason.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		if reason.IsProvisioningWait(err) {
			mg.SetConditions(xpv1.Unavailable())
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
		}
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	exp, err := e.Client.ExportInstance(ctx, name)
	if err != nil {
		if reason.IsProvisioningWait(err) {
			mg.SetConditions(xpv1.Unavailable())
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
		}
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	actual, err := apiToSpec(ai, exp)
	if err != nil {
		wrap := fmt.Errorf("could not transform instance from Akuity API: %w", err)
		mg.SetConditions(xpv1.ReconcileError(wrap))
		return managed.ExternalObservation{}, wrap
	}

	// Sensitive-ref fields live in spec only (the gateway masks Secret
	// contents on read and returns nothing at all for *Ref shapes).
	// Copy the user-declared refs verbatim into actual so the struct
	// comparison ignores them; rotation of the backing Secret is still
	// detected downstream via the secret-hash annotation.
	carryOverSensitiveRefs(&mg.Spec.ForProvider, &actual)

	lateInitialized := lateInitialize(&mg.Spec.ForProvider, &actual)

	// Preserve SecretHash across the AtProvider refresh. apiToObservation
	// rebuilds the full observation struct from gateway data, which has
	// no SecretHash field; without this carry-over every Observe would
	// see an empty hash and refuse to detect rotation until the next
	// Create/Update wrote a new value.
	prevSecretHash := mg.Status.AtProvider.SecretHash
	mg.Status.AtProvider = apiToObservation(ai, exp)
	mg.Status.AtProvider.SecretHash = prevSecretHash

	if mg.Status.AtProvider.HealthStatus.Code != 1 {
		mg.SetConditions(xpv1.Unavailable())
	} else {
		mg.SetConditions(xpv1.Available())
	}

	upToDate := isUpToDate(mg.Spec.ForProvider, actual)
	if !upToDate {
		e.Logger.Debug("Instance drift detected", "diff", cmp.Diff(mg.Spec.ForProvider, actual))
	}

	// ArgocdResources drift runs independently of the struct-level
	// comparison because the wire and spec shapes are not symmetric
	// (see isUpToDate for the long version). Additive semantics: we
	// only fire drift for desired children that are missing or
	// divergent server-side; extra server-side children are ignored
	// on purpose so removing an entry from spec does not delete it
	// on the gateway.
	if upToDate {
		ok, report, rerr := argocdResourcesUpToDate(mg.Spec.ForProvider.Resources, exp)
		if rerr != nil {
			mg.SetConditions(xpv1.ReconcileError(rerr))
			return managed.ExternalObservation{}, rerr
		}
		if !ok {
			e.Logger.Debug("argocdResources drift detected",
				"missing", report.Missing, "changed", report.Changed)
			upToDate = false
		}
	}

	// Secrets are masked on GetInstance, so the only cheap way to detect
	// rotation of a referenced kube Secret is to re-resolve on every
	// Observe and compare the digest against the last applied value
	// recorded on the managed resource. A mismatch forces Update to fire
	// a fresh Apply, propagating the new secret material server-side.
	if upToDate {
		sec, serr := resolveInstanceSecrets(ctx, e.Kube, mg)
		if serr != nil {
			mg.SetConditions(xpv1.ReconcileError(serr))
			return managed.ExternalObservation{}, serr
		}
		if sec.Hash() != getSecretHash(mg) {
			e.Logger.Debug("Instance secret hash changed; forcing re-Apply",
				"previous", getSecretHash(mg), "current", sec.Hash())
			upToDate = false
		}
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        upToDate,
		ResourceLateInitialized: lateInitialized,
	}, nil
}

func (e *external) Create(ctx context.Context, mg *v1alpha2.Instance) (managed.ExternalCreation, error) {
	sec, err := resolveInstanceSecrets(ctx, e.Kube, mg)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	req, err := buildApplyRequest(ctx, e.Client, mg, sec)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	if err := e.Client.ApplyInstance(ctx, req); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, req.GetId())
	setSecretHash(mg, sec.Hash())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha2.Instance) (managed.ExternalUpdate, error) {
	sec, err := resolveInstanceSecrets(ctx, e.Kube, mg)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	req, err := buildApplyRequest(ctx, e.Client, mg, sec)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	if err := e.Client.ApplyInstance(ctx, req); err != nil {
		return managed.ExternalUpdate{}, err
	}
	setSecretHash(mg, sec.Hash())
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg *v1alpha2.Instance) (managed.ExternalDelete, error) {
	name := meta.GetExternalName(mg)
	if name == "" {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, e.Client.DeleteInstance(ctx, name)
}

func (e *external) Disconnect(ctx context.Context) error { return nil }
