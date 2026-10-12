package reason_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

func TestAsTerminal(t *testing.T) {
	err := errors.New("bad input")
	tw := reason.AsTerminal(err)
	assert.Equal(t, err.Error(), tw.Error())
	assert.True(t, reason.IsTerminal(tw))
	assert.ErrorIs(t, tw, err)
}

func TestIsTerminal_NotWrapped(t *testing.T) {
	assert.False(t, reason.IsTerminal(errors.New("plain")))
	assert.False(t, reason.IsTerminal(nil))
}

func TestClassifyApplyError_BadInputWrapsAsTerminal(t *testing.T) {
	got := reason.ClassifyApplyError(status.Error(codes.InvalidArgument, "reserved key admin.password"))
	require.Error(t, got)
	assert.True(t, reason.IsTerminal(got))
}

func TestClassifyApplyError_PermissionDeniedWrapsAsTerminal(t *testing.T) {
	got := reason.ClassifyApplyError(status.Error(codes.PermissionDenied, "Access to instance shared-inst is denied"))
	require.Error(t, got)
	assert.True(t, reason.IsTerminal(got))
}

func TestClassifyApplyError_NilPassthrough(t *testing.T) {
	assert.NoError(t, reason.ClassifyApplyError(nil))
}

func TestClassifyApplyError_ProvisioningWaitStaysRetryable(t *testing.T) {
	got := reason.ClassifyApplyError(status.Error(codes.InvalidArgument, "instance is still being provisioned"))
	require.Error(t, got)
	assert.False(t, reason.IsTerminal(got))
	assert.True(t, reason.IsRetryable(got))
}

func TestClassifyApplyError_TransientStaysRetryable(t *testing.T) {
	got := reason.ClassifyApplyError(status.Error(codes.Unavailable, "transient"))
	require.Error(t, got)
	assert.False(t, reason.IsTerminal(got))
	assert.True(t, reason.IsRetryable(got))
}

func TestClassifyApplyError_NonGRPCPassesThrough(t *testing.T) {
	plain := errors.New("plain")
	got := reason.ClassifyApplyError(plain)
	assert.Same(t, plain, got)
}

func TestClassifyApplyError_AlreadyTerminalIsIdempotent(t *testing.T) {
	tw := reason.AsTerminal(errors.New("already classified"))
	got := reason.ClassifyApplyError(tw)
	assert.True(t, reason.IsTerminal(got))
}

func TestClassifyManifestInstallError_NotReconciledStaysRetryable(t *testing.T) {
	got := reason.ClassifyManifestInstallError(reason.AsNotReconciled(errors.New("cluster has not yet been reconciled")))
	require.Error(t, got)
	assert.False(t, reason.IsTerminal(got))
	assert.True(t, reason.IsRetryable(got))
}

func TestClassifyManifestInstallError_FailedPreconditionStaysRetryable(t *testing.T) {
	got := reason.ClassifyManifestInstallError(status.Error(codes.FailedPrecondition, "cluster has not yet been reconciled"))
	require.Error(t, got)
	assert.False(t, reason.IsTerminal(got))
	assert.True(t, reason.IsRetryable(got))
}

func TestClassifyManifestInstallError_OtherwiseTerminal(t *testing.T) {
	got := reason.ClassifyManifestInstallError(errors.New("bad kubeconfig"))
	require.Error(t, got)
	assert.True(t, reason.IsTerminal(got))
}

// A FailedPrecondition from Apply is never terminal. The agent version
// pin conflict is the motivating case: it is cleared on the parent
// Instance or KargoInstance, which the child's per-resource write guard
// cannot observe, so caching it would leave the child Synced=False until
// its spec changes or the process restarts. The platform also answers
// FailedPrecondition for transient states such as "Resource is still in
// use", which must keep retrying for the same reason.
func TestClassifyApplyError_FailedPreconditionNotTerminal(t *testing.T) {
	for _, msg := range []string{
		"agent version is pinned to 0.5.88 in the instance settings, unpin it to change agent versions",
		"Resource is still in use",
	} {
		got := reason.ClassifyApplyError(status.Error(codes.FailedPrecondition, msg))
		require.Error(t, got, msg)
		assert.False(t, reason.IsTerminal(got), msg)
	}
}
