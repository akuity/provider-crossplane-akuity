// Package ratelimit builds the global rate limiter shared by every Akuity
// controller. It is a token bucket that caps the total rate of reconciles
// across the provider process, the back-pressure against the Akuity API.
//
// The limiter is passed to crossplane-runtime's rate-limited reconciler
// wrapper, which asks it for a delay before every reconcile. It must not
// carry a per-item component: a per-item limiter answers with a non-zero
// delay on its first call, the wrapper turns that into RequeueAfter, and
// controller-runtime then forgets the workqueue's own exponential backoff.
// The result is a persistently failing resource that retries every few
// seconds instead of backing off. Per-item exponential backoff already
// comes from the workqueue limiter that controller.Options installs.
package ratelimit

import (
	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"

	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
)

// Default rates. Conservative values, expected to be tuned against staging load.
const (
	DefaultRPS        = 10
	DefaultBurstRatio = 10 // burst = rps * BurstRatio
)

// ForAkuity returns the global token bucket for Akuity managed reconcilers.
// rps bounds the steady-state reconcile rate across all controllers sharing
// this limiter. Caller passes the same instance to every controller so the
// budget is shared. Passing rps <= 0 falls back to DefaultRPS.
func ForAkuity(rps int) ratelimiter.RateLimiter {
	if rps <= 0 {
		rps = DefaultRPS
	}
	return &workqueue.TypedBucketRateLimiter[string]{
		Limiter: rate.NewLimiter(rate.Limit(rps), rps*DefaultBurstRatio),
	}
}
