// Package ratelimit builds the workqueue rate limiter used by every Akuity
// controller. It combines two behaviors:
//
//  1. A global token bucket that caps the total rate of reconciles across all
//     controllers in the provider process. This is the back-pressure against
//     the Akuity API.
//  2. A per-item exponential backoff so a single persistently-failing managed
//     resource does not hot-loop against the API.
//
// The limiter returned by ForAkuity is installed into every managed reconciler
// via controller-runtime's controller.Options.RateLimiter field.
package ratelimit

import (
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"

	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
)

// Default rates. Conservative values, expected to be tuned against staging load.
const (
	DefaultRPS        = 10
	DefaultBurstRatio = 10 // burst = rps * BurstRatio
	DefaultBaseDelay  = 1 * time.Second
	DefaultMaxDelay   = 60 * time.Second
)

// ForAkuity returns a workqueue rate limiter for Akuity managed reconcilers.
// rps bounds the steady-state reconcile rate across all controllers sharing
// this limiter. Caller passes the same instance to every controller so the
// budget is shared.
//
// Passing rps <= 0 falls back to DefaultRPS. The per-item exponential backoff
// runs from DefaultBaseDelay to DefaultMaxDelay regardless of rps.
//
// The return type matches crossplane-runtime's ratelimiter.RateLimiter
// (workqueue.TypedRateLimiter[string]) so the result can be passed directly
// to controller.Options.GlobalRateLimiter.
func ForAkuity(rps int) ratelimiter.RateLimiter {
	if rps <= 0 {
		rps = DefaultRPS
	}
	global := &workqueue.TypedBucketRateLimiter[string]{
		Limiter: rate.NewLimiter(rate.Limit(rps), rps*DefaultBurstRatio),
	}
	perItem := workqueue.NewTypedItemExponentialFailureRateLimiter[string](
		DefaultBaseDelay, DefaultMaxDelay,
	)
	return workqueue.NewTypedMaxOfRateLimiter[string](perItem, global)
}
