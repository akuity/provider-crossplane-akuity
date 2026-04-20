package metrics

import (
	"context"
	"errors"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// NewLegacyDeprecationInitializer returns an Initializer that records a
// Warning event on every reconcile of a v1alpha1 managed resource. Wiring as
// an Initializer (rather than inside the External client) keeps the
// deprecation notice on the top-level reconcile loop — so users see it on
// the CR's .status even when the external client short-circuits (e.g. on
// Connect failures or when the resource is being deleted).
func NewLegacyDeprecationInitializer(r event.Recorder) managed.Initializer {
	return managed.InitializerFn(func(_ context.Context, mg resource.Managed) error {
		r.Event(mg, event.Warning(LegacyDeprecationReason, errors.New(LegacyDeprecationMessage)))
		return nil
	})
}
