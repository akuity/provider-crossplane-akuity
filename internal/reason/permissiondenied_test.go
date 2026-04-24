package reason_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

func TestAsPermissionDenied(t *testing.T) {
	err := errors.New("forbidden")
	pd := reason.AsPermissionDenied(err)
	assert.Equal(t, err.Error(), pd.Error())
}

func TestIsPermissionDenied(t *testing.T) {
	assert.True(t, reason.IsPermissionDenied(reason.AsPermissionDenied(errors.New("forbidden"))))
	assert.False(t, reason.IsPermissionDenied(errors.New("another error")))
	assert.False(t, reason.IsPermissionDenied(reason.AsNotFound(errors.New("not found"))))
}
