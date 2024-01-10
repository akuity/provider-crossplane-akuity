package pointer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akuityio/provider-crossplane-akuity/internal/utils/pointer"
)

// If current is the default value, we should return from.
func TestLateInitialize_IsDefault(t *testing.T) {
	currentStr := ""
	fromStr := "default"
	assert.Equal(t, fromStr, pointer.LateInitialize(currentStr, fromStr))

	currentInt := 0
	fromInt := 42
	assert.Equal(t, fromInt, pointer.LateInitialize(currentInt, fromInt))
}

// If current is not the default value, we should return current.
func TestLateInitialize_IsNotDefault(t *testing.T) {
	currentStr := "not-default"
	fromStr := "default"
	assert.Equal(t, currentStr, pointer.LateInitialize(currentStr, fromStr))

	currentInt := 10
	fromInt := 42
	assert.Equal(t, currentInt, pointer.LateInitialize(currentInt, fromInt))
}
