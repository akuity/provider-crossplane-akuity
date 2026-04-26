package reason

import (
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Retryable struct {
	error
}

func AsRetryable(err error) Retryable {
	return Retryable{err}
}

func IsRetryable(err error) bool {
	if errors.Is(err, Retryable{}) {
		return true
	}
	return isRetryableGRPC(err)
}

// IsProvisioningWait returns true when err signals that the target
// Akuity resource is still being provisioned. Controllers use this to
// mark the managed resource Unavailable in Observe without escalating
// to ReconcileError; the gRPC error is a transient wait-state, not a
// reconcile failure.
func IsProvisioningWait(err error) bool {
	if err == nil {
		return false
	}
	s, ok := status.FromError(err)
	if !ok {
		return false
	}
	if s.Code() != codes.InvalidArgument {
		return false
	}
	return strings.Contains(s.Message(), "still being provisioned")
}

func (r Retryable) Is(err error) bool {
	return err == Retryable{}
}

// isRetryableGRPC classifies raw gRPC errors that callers did not wrap with
// AsRetryable. It recognises the transient status codes used by the Akuity
// gateway plus common network-level substrings.
func isRetryableGRPC(err error) bool {
	if err == nil {
		return false
	}
	if s, ok := status.FromError(err); ok {
		switch s.Code() {
		case codes.Unavailable,
			codes.DeadlineExceeded,
			codes.Aborted,
			codes.ResourceExhausted,
			codes.Internal:
			return true
		case codes.InvalidArgument:
			// Akuity API returns InvalidArgument with this substring while a
			// resource is still being provisioned.
			if strings.Contains(s.Message(), "still being provisioned") {
				return true
			}
		case codes.OK, codes.Canceled, codes.Unknown, codes.NotFound,
			codes.AlreadyExists, codes.PermissionDenied, codes.FailedPrecondition,
			codes.OutOfRange, codes.Unimplemented, codes.DataLoss, codes.Unauthenticated:
			// Terminal codes: not retryable at the gRPC layer. The
			// substring probe below still handles network-level strings
			// that arrive as generic errors.
		}
	}
	msg := err.Error()
	for _, sub := range []string{
		"connection refused",
		"connection reset",
		"no such host",
		"i/o timeout",
		"EOF",
	} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}
