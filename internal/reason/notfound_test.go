package reason_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

func TestAsNotFound(t *testing.T) {
	err := errors.New("not found")
	notFound := reason.AsNotFound(err)
	assert.Equal(t, err.Error(), notFound.Error())
}

func TestIsNotFound(t *testing.T) {
	assert.True(t, reason.IsNotFound(reason.AsNotFound(errors.New("not found"))))
	assert.False(t, reason.IsNotFound(errors.New("another error")))
}
