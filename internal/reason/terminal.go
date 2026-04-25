package reason

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Terminal classifies an Apply-side error that the user must fix in the
// spec or referenced Secret before reconciliation can make progress.
// Wrapping with Terminal lets call sites that want to distinguish
// "transient gateway error worth retrying" from "bad input" do so via
// IsTerminal without re-parsing the underlying gRPC status.
//
// The current code does not stop the controller-runtime requeue itself
// (the managed reconciler still polls at PollInterval); the wrapper is
// load-bearing for: (a) the Synced=False condition message that surfaces
// the platform's reason to the user, and (b) downstream callers that
// want to short-circuit retries on a Terminal-classified error.
type Terminal struct {
	error
}

func AsTerminal(err error) Terminal {
	return Terminal{err}
}

func IsTerminal(err error) bool {
	return errors.Is(err, Terminal{})
}

func (t Terminal) Is(err error) bool {
	return err == Terminal{}
}

func (t Terminal) Unwrap() error {
	return t.error
}

// ClassifyApplyError tags Apply-path gRPC errors so callers can
// distinguish transient retry-worthy failures from terminal "bad input"
// failures the user must fix on the spec / Secret. Non-gRPC errors and
// already-wrapped errors pass through unchanged.
//
// codes.InvalidArgument outside of the "still being provisioned" wait
// pattern is the canonical bad-input signal from the Akuity gateway
// (reserved keys in admin/server SecretRefs, malformed credentials,
// missing Kargo project namespaces). Without classification, every
// reconcile poll would keep firing Apply against the same bad payload
// and hot-loop write traffic against portal-server until the user
// mutates the spec.
func ClassifyApplyError(err error) error {
	if err == nil {
		return nil
	}
	if IsTerminal(err) || IsRetryable(err) {
		return err
	}
	s, ok := status.FromError(err)
	if !ok {
		return err
	}
	if s.Code() == codes.InvalidArgument {
		return AsTerminal(err)
	}
	return err
}
