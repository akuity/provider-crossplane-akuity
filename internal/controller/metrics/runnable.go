package metrics

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
)

// DefaultLegacyCountInterval is how often LegacyCounter relists v1alpha1
// managed resources when the caller does not supply a value. Short enough
// that the gauge tracks operator action; long enough that the List load is
// negligible on a cluster with thousands of CRs.
const DefaultLegacyCountInterval = 60 * time.Second

// LegacyCounter is a controller-runtime Runnable that periodically counts
// the v1alpha1 managed resources on the API server and updates LegacyCRCount
// accordingly. Runs only on the leader so gauges don't double-count from
// standby replicas.
type LegacyCounter struct {
	Client   client.Client
	Interval time.Duration
	Log      logging.Logger
}

// NeedLeaderElection makes LegacyCounter leader-scoped — multiple replicas
// would otherwise publish competing values for the same gauge series.
func (l *LegacyCounter) NeedLeaderElection() bool { return true }

// Start runs the counter loop until ctx is cancelled. Satisfies
// manager.Runnable.
func (l *LegacyCounter) Start(ctx context.Context) error {
	interval := l.Interval
	if interval <= 0 {
		interval = DefaultLegacyCountInterval
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	l.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			l.tick(ctx)
		}
	}
}

func (l *LegacyCounter) tick(ctx context.Context) {
	var clusters v1alpha1.ClusterList
	if err := l.Client.List(ctx, &clusters); err != nil {
		l.Log.Debug("list v1alpha1 clusters for legacy-CR gauge", "error", err)
	} else {
		LegacyCRCount.WithLabelValues("Cluster").Set(float64(len(clusters.Items)))
	}

	var instances v1alpha1.InstanceList
	if err := l.Client.List(ctx, &instances); err != nil {
		l.Log.Debug("list v1alpha1 instances for legacy-CR gauge", "error", err)
	} else {
		LegacyCRCount.WithLabelValues("Instance").Set(float64(len(instances.Items)))
	}
}

// SetupLegacyTelemetry registers the LegacyCounter runnable on the supplied
// manager. Call once from the top-level controller Setup chain.
func SetupLegacyTelemetry(mgr manager.Manager, log logging.Logger) error {
	return mgr.Add(&LegacyCounter{
		Client:   mgr.GetClient(),
		Interval: DefaultLegacyCountInterval,
		Log:      log,
	})
}
