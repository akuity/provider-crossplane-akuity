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
		//exhaustive:ignore // only classifying the transient subset; everything else falls through to the substring probe below.
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
