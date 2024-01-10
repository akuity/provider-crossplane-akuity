package reason_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

func TestAsNotReconciled(t *testing.T) {
	err := errors.New("some error")
	notReconciled := reason.AsNotReconciled(err)
	assert.Equal(t, err.Error(), notReconciled.Error())
}

func TestIsNotReconciled(t *testing.T) {
	err := reason.AsNotReconciled(errors.New("some error"))
	assert.True(t, reason.IsNotReconciled(err))
	assert.False(t, reason.IsNotReconciled(errors.New("another error")))
}
