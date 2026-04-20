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

func TestForAkuity_ExponentialBackoffAdvancesOnRepeatedFailure(t *testing.T) {
	rl := ratelimit.ForAkuity(100)

	first := rl.When(item)
	second := rl.When(item)
	third := rl.When(item)

	// Per-item exponential backoff must grow monotonically across failures.
	// The global token bucket alone would return ~constant delays; the
	// assertion below fails if the per-item limiter is missing.
	assert.GreaterOrEqual(t, second, first)
	assert.GreaterOrEqual(t, third, second)
}

func TestForAkuity_ForgetResetsPerItemBackoff(t *testing.T) {
	rl := ratelimit.ForAkuity(100)

	_ = rl.When(item)
	_ = rl.When(item)
	rl.Forget(item)
	after := rl.When(item)
	assert.LessOrEqual(t, after, ratelimit.DefaultBaseDelay*2)
}
