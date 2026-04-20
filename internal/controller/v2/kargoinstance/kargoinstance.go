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

// Package kargoinstance is the v1alpha2 KargoInstance controller. It
// drives the Akuity Kargo-plane ApplyKargoInstance endpoint through
// the Kargo service gateway configured on internal/clients/akuity.
package kargoinstance

import (
	"context"
	"fmt"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	apisv1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
)

// Setup registers the controller with the manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha2.KargoInstanceGroupKind)
	logger := o.Logger.WithValues("controller", name)
	recorder := event.NewAPIRecorder(mgr.GetEventRecorderFor(name))

	conn := &base.Connector[*v1alpha2.KargoInstance]{
		Kube:      mgr.GetClient(),
		Usage:     resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha2.ProviderConfigUsage{}),
		Logger:    logger,
		Recorder:  recorder,
		NewClient: base.DefaultClientFactory,
		Build: func(ac akuity.Client, kube client.Client, l logging.Logger, r event.Recorder) managed.TypedExternalClient[*v1alpha2.KargoInstance] {
			return &external{ExternalClient: base.NewExternalClient(ac, kube, l, r)}
		},
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.KargoInstanceGroupVersionKind),
		managed.WithTypedExternalConnector[*v1alpha2.KargoInstance](conn),
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.KargoInstance{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type external struct {
	base.ExternalClient
}

func (e *external) Observe(ctx context.Context, mg *v1alpha2.KargoInstance) (managed.ExternalObservation, error) {
	if meta.GetExternalName(mg) == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	ki, err := e.Client.GetKargoInstance(ctx, meta.GetExternalName(mg))
	if err != nil {
		if reason.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	actual, err := apiToSpec(ki)
	if err != nil {
		mg.SetConditions(xpv1.ReconcileError(err))
		return managed.ExternalObservation{}, err
	}

	mg.Status.AtProvider = apiToObservation(ki)
	if mg.Status.AtProvider.HealthStatus.Code != 1 {
		mg.SetConditions(xpv1.Unavailable())
	} else {
		mg.SetConditions(xpv1.Available())
	}

	upToDate := cmp.Equal(mg.Spec.ForProvider, actual)
	if !upToDate {
		e.Logger.Debug("KargoInstance drift detected", "diff", cmp.Diff(mg.Spec.ForProvider, actual))
	}
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(ctx context.Context, mg *v1alpha2.KargoInstance) (managed.ExternalCreation, error) {
	if err := e.apply(ctx, mg); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(mg, mg.Spec.ForProvider.Name)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg *v1alpha2.KargoInstance) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, e.apply(ctx, mg)
}

func (e *external) Delete(ctx context.Context, mg *v1alpha2.KargoInstance) (managed.ExternalDelete, error) {
	name := meta.GetExternalName(mg)
	if name == "" {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, e.Client.DeleteKargoInstance(ctx, name)
}

func (e *external) Disconnect(ctx context.Context) error { return nil }

// apply is shared by Create and Update.
func (e *external) apply(ctx context.Context, mg *v1alpha2.KargoInstance) error {
	kargoPB, err := specToPB(mg.Spec.ForProvider)
	if err != nil {
		return err
	}
	req := &kargov1.ApplyKargoInstanceRequest{
		IdType: idv1.Type_NAME,
		Id:     mg.Spec.ForProvider.Name,
		Kargo:  kargoPB,
	}
	return e.Client.ApplyKargoInstance(ctx, req)
}

// specToPB marshals the curated v1alpha2 KargoInstance into the
// structpb.Struct shape the Kargo ApplyKargoInstance endpoint expects
// under the "kargo" field. The conversion goes through the generated
// KargoSpec converter then through the JSON→map→structpb bridge
// provided by internal/marshal.
func specToPB(in v1alpha2.KargoInstanceParameters) (*structpb.Struct, error) {
	wire := akuitytypes.Kargo{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Kargo",
			APIVersion: "kargo.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: in.Name},
	}
	if s := convert.KargoSpecSpecToAPI(&in.Spec); s != nil {
		wire.Spec = *s
	}
	pb, err := marshal.APIModelToPBStruct(wire)
	if err != nil {
		return nil, fmt.Errorf("marshal kargo instance: %w", err)
	}
	return pb, nil
}
