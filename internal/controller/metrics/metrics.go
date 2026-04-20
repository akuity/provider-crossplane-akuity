// Package metrics exposes observational telemetry for the v1alpha1 →
// v1alpha2 deprecation window. Nothing here gates controller behaviour; the
// 6-month removal timeline is calendar-driven (see §5 and §7 of
// REFACTOR_PLAN.md).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// LegacyRemovalVersion is the provider release that removes the
// core.akuity.crossplane.io/v1alpha1 API surface.
const LegacyRemovalVersion = "v3.0.0"

// LegacyRemovalTargetDate is the scheduled v3.0.0 cut date — 6 calendar
// months after the v2.0.0 cutover. Update this once v2.0.0 ships.
const LegacyRemovalTargetDate = "2026-10-20"

// LegacyDeprecationReason is the Event reason recorded on every reconcile of
// a v1alpha1 managed resource.
const LegacyDeprecationReason = "Deprecated"

// LegacyDeprecationMessage is the Warning-event message emitted on every
// reconcile of a v1alpha1 managed resource. The message points users at the
// v1alpha2 namespaced API group.
const LegacyDeprecationMessage = "core.akuity.crossplane.io/v1alpha1 is deprecated and will be removed in " +
	LegacyRemovalVersion + " (target " + LegacyRemovalTargetDate +
	"); migrate to core.m.akuity.crossplane.io/v1alpha2"

// LegacyCRCount reports the number of v1alpha1 managed-resource instances
// currently on the API server, labelled by kind. Observational only; used to
// inform release communications, not to gate removal.
var LegacyCRCount = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "akuity_legacy_v1alpha1_cr_count",
		Help: "Number of core.akuity.crossplane.io/v1alpha1 managed resources on the API server, labelled by kind.",
	},
	[]string{"kind"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(LegacyCRCount)
}
