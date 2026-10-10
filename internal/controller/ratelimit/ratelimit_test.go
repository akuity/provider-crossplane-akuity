package ratelimit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akuityio/provider-crossplane-akuity/internal/controller/ratelimit"
)

const item = "test-controller/test-resource"

func TestForAkuity_FallsBackToDefaultWhenNonPositive(t *testing.T) {
	rl := ratelimit.ForAkuity(0)
	assert.NotNil(t, rl)
	// Exercise the limiter once to ensure it doesn't panic.
	_ = rl.When(item)
}

// The global limiter must answer zero while the bucket has tokens. Any
// per-item delay here is turned into RequeueAfter by the rate-limited
// reconciler wrapper, which resets the workqueue's exponential backoff
// and makes a persistently failing resource retry every few seconds.
func TestForAkuity_NoPerItemDelayWhileBucketHasTokens(t *testing.T) {
	rl := ratelimit.ForAkuity(100)

	for i := 0; i < 5; i++ {
		assert.Zero(t, rl.When(item), "call %d", i)
	}
}
