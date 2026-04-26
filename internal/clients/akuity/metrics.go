// Package akuity exposes test-tooling instrumentation.
//
// apiWriteCallsTotal is incremented at every write-path Akuity gateway
// call so validation runs can diff /metrics windows and detect repeated
// Apply loops.
package akuity

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// apiWriteCallsTotal counts every write-path gateway call the Akuity
// client makes. Labels:
//   - method:      gRPC method name on the gateway (e.g. "ApplyInstance",
//     "PatchKargoInstance", "CreateKargoInstanceAgent").
//   - resource_id: the canonical Akuity ID (or name, if that is what the
//     caller passes in) the call targets.
var apiWriteCallsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "akuity_api_client_writes_total",
		Help: "Write-path (Apply/Patch/Create/Update/Delete) calls the Akuity client issued to the gateway, labelled by gRPC method and target resource id.",
	},
	[]string{"method", "resource_id"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(apiWriteCallsTotal)
}

// incAPIWrite bumps the write-call counter. Kept as a package-level
// helper so the instrumentation at each call site is a single line.
func incAPIWrite(method, resourceID string) {
	apiWriteCallsTotal.WithLabelValues(method, resourceID).Inc()
}
