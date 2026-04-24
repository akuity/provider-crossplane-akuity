package reason_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

func TestAsRetryable(t *testing.T) {
	err := errors.New("transient")
	r := reason.AsRetryable(err)
	assert.Equal(t, err.Error(), r.Error())
	assert.True(t, reason.IsRetryable(r))
}

func TestIsRetryable_gRPCCodes(t *testing.T) {
	cases := []struct {
		code     codes.Code
		msg      string
		expected bool
	}{
		{codes.Unavailable, "service unavailable", true},
		{codes.DeadlineExceeded, "deadline", true},
		{codes.Aborted, "aborted", true},
		{codes.ResourceExhausted, "throttled", true},
		{codes.Internal, "internal", true},
		{codes.InvalidArgument, "instance is still being provisioned", true},
		{codes.InvalidArgument, "bad request", false},
		{codes.NotFound, "not found", false},
		{codes.PermissionDenied, "denied", false},
		{codes.OK, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.code.String()+"/"+tc.msg, func(t *testing.T) {
			err := status.Error(tc.code, tc.msg)
			assert.Equal(t, tc.expected, reason.IsRetryable(err))
		})
	}
}

func TestIsRetryable_NetworkSubstrings(t *testing.T) {
	cases := []string{
		"dial tcp: connection refused",
		"read: connection reset by peer",
		"no such host",
		"context: i/o timeout",
		"unexpected EOF",
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			assert.True(t, reason.IsRetryable(errors.New(msg)))
		})
	}
}

func TestIsRetryable_NonRetryable(t *testing.T) {
	assert.False(t, reason.IsRetryable(errors.New("validation failed")))
	assert.False(t, reason.IsRetryable(nil))
}
