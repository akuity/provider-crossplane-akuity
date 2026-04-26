/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package base_test

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

func TestClassifyGetError_OK(t *testing.T) {
	outcome, obs, err := base.ClassifyGetError(nil)
	assert.Equal(t, base.GetOK, outcome)
	assert.Equal(t, managed.ExternalObservation{}, obs)
	assert.NoError(t, err)
}

func TestClassifyGetError_NotFound(t *testing.T) {
	outcome, obs, err := base.ClassifyGetError(reason.AsNotFound(errors.New("missing")))
	assert.Equal(t, base.GetAbsent, outcome)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: false}, obs)
	assert.NoError(t, err, "absent is not a retryable error; caller returns obs with nil err")
}

func TestClassifyGetError_ProvisioningWait(t *testing.T) {
	// reason.IsProvisioningWait keys off codes.InvalidArgument + the
	// "still being provisioned" substring; match that shape.
	pw := status.Error(codes.InvalidArgument, "instance still being provisioned")
	outcome, obs, err := base.ClassifyGetError(pw)
	assert.Equal(t, base.GetProvisioning, outcome)
	assert.True(t, obs.ResourceExists)
	assert.True(t, obs.ResourceUpToDate, "provisioning must report up-to-date so reconcile does not thrash on a parent still coming up")
	assert.NoError(t, err, "provisioning is not terminal; caller parks reconcile with obs")
}

func TestClassifyGetError_Terminal(t *testing.T) {
	boom := errors.New("boom")
	outcome, obs, err := base.ClassifyGetError(boom)
	assert.Equal(t, base.GetTerminal, outcome)
	assert.Equal(t, managed.ExternalObservation{}, obs)
	assert.ErrorIs(t, err, boom, "terminal errors pass through unwrapped so managed.Reconciler's ReconcileError captures the original cause")
}
