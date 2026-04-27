// Package akuity exposes optional validation instrumentation.
//
// apiWriteCallsTotal is incremented at every write-path Akuity gateway
// call so validation runs can diff /metrics windows and detect repeated
// Apply loops. It is disabled by default and only registered when
// AKUITY_PROVIDER_ENABLE_VALIDATION_METRICS=true is set before startup.
package akuity

import (
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const enableValidationMetricsEnv = "AKUITY_PROVIDER_ENABLE_VALIDATION_METRICS"

// apiWriteCallsTotal counts every write-path gateway call the Akuity
// client makes. Labels:
//   - method:      gRPC method name on the gateway (e.g. "ApplyInstance",
//     "PatchKargoInstance", "CreateKargoInstanceAgent").
//   - resource_id: the canonical Akuity ID (or name, if that is what the
//     caller passes in) the call targets.
var apiWriteCallsTotal *prometheus.CounterVec

func init() {
	if !strings.EqualFold(os.Getenv(enableValidationMetricsEnv), "true") {
		return
	}

	apiWriteCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "akuity_api_client_writes_total",
			Help: "Write-path (Apply/Patch/Create/Update/Delete) calls the Akuity client issued to the gateway, labelled by gRPC method and target resource id.",
		},
		[]string{"method", "resource_id"},
	)
	ctrlmetrics.Registry.MustRegister(apiWriteCallsTotal)
}

// incAPIWrite bumps the write-call counter when validation metrics are enabled.
func incAPIWrite(method, resourceID string) {
	if apiWriteCallsTotal == nil {
		return
	}
	apiWriteCallsTotal.WithLabelValues(method, resourceID).Inc()
}
