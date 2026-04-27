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
	"errors"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

func TestTerminalWriteGuardSuppressesSameTerminalPayload(t *testing.T) {
	guard := base.NewTerminalWriteGuard()
	mg := terminalGuardInstance(1)
	key, err := base.NewTerminalWriteKey(mg, v1alpha1.InstanceGroupVersionKind, map[string]string{"name": "bad"})
	require.NoError(t, err)

	terminalErr := reason.AsTerminal(errors.New("bad payload"))
	guard.Record(key, terminalErr)

	assert.True(t, guard.HasResource(base.NewTerminalWriteResourceKey(mg, v1alpha1.InstanceGroupVersionKind)))

	obs, err, ok := guard.Suppress(mg, key)
	assert.True(t, ok)
	require.Error(t, err)
	assert.Equal(t, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, obs)
	require.Len(t, mg.Status.Conditions, 1)
	assert.Equal(t, xpv1.ReasonReconcileError, mg.Status.Conditions[0].Reason)
	assert.Contains(t, mg.Status.Conditions[0].Message, "bad payload")
}

func TestTerminalWriteGuardIgnoresNonTerminalError(t *testing.T) {
	guard := base.NewTerminalWriteGuard()
	mg := terminalGuardInstance(1)
	key, err := base.NewTerminalWriteKey(mg, v1alpha1.InstanceGroupVersionKind, "payload")
	require.NoError(t, err)

	guard.Record(key, errors.New("transient"))

	_, _, ok := guard.Suppress(mg, key)
	assert.False(t, ok)
}

func TestTerminalWriteGuardClearsWhenPayloadChanges(t *testing.T) {
	guard := base.NewTerminalWriteGuard()
	mg := terminalGuardInstance(1)
	oldKey, err := base.NewTerminalWriteKey(mg, v1alpha1.InstanceGroupVersionKind, "old")
	require.NoError(t, err)
	newKey, err := base.NewTerminalWriteKey(mg, v1alpha1.InstanceGroupVersionKind, "new")
	require.NoError(t, err)

	guard.Record(oldKey, reason.AsTerminal(errors.New("bad payload")))

	_, _, ok := guard.Suppress(mg, newKey)
	assert.False(t, ok)

	_, _, ok = guard.Suppress(mg, oldKey)
	assert.False(t, ok, "changed payload should clear the stale terminal record")
}

func TestTerminalWriteGuardSuppressesWhenOnlyGenerationChanges(t *testing.T) {
	guard := base.NewTerminalWriteGuard()
	mg := terminalGuardInstance(1)
	oldKey, err := base.NewTerminalWriteKey(mg, v1alpha1.InstanceGroupVersionKind, "payload")
	require.NoError(t, err)
	guard.Record(oldKey, reason.AsTerminal(errors.New("bad payload")))

	mg.SetGeneration(2)
	newKey, err := base.NewTerminalWriteKey(mg, v1alpha1.InstanceGroupVersionKind, "payload")
	require.NoError(t, err)

	_, _, ok := guard.Suppress(mg, newKey)
	assert.True(t, ok)

	mg.SetGeneration(1)
	_, _, ok = guard.Suppress(mg, oldKey)
	assert.True(t, ok, "same payload should remain suppressed across generation-only churn")
}

func TestTerminalWriteGuardFallsBackToNamespacedNameWhenUIDEmpty(t *testing.T) {
	guard := base.NewTerminalWriteGuard()
	mg := terminalGuardInstance(1)
	mg.SetUID("")
	mg.SetNamespace("default")
	oldKey, err := base.NewTerminalWriteKey(mg, v1alpha1.InstanceGroupVersionKind, "payload")
	require.NoError(t, err)
	guard.Record(oldKey, reason.AsTerminal(errors.New("bad payload")))

	next := terminalGuardInstance(1)
	next.SetUID("")
	next.SetNamespace("default")
	newKey, err := base.NewTerminalWriteKey(next, v1alpha1.InstanceGroupVersionKind, "payload")
	require.NoError(t, err)

	_, _, ok := guard.Suppress(next, newKey)
	assert.True(t, ok)
}

func TestTerminalWriteGuardClearsByResourceKey(t *testing.T) {
	guard := base.NewTerminalWriteGuard()
	mg := terminalGuardInstance(1)
	key, err := base.NewTerminalWriteKey(mg, v1alpha1.InstanceGroupVersionKind, "payload")
	require.NoError(t, err)
	guard.Record(key, reason.AsTerminal(errors.New("bad payload")))

	guard.Clear(base.NewTerminalWriteResourceKey(mg, v1alpha1.InstanceGroupVersionKind))

	_, _, ok := guard.Suppress(mg, key)
	assert.False(t, ok)
}

func terminalGuardInstance(generation int64) *v1alpha1.Instance {
	return &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "example",
			UID:        k8stypes.UID("instance-uid"),
			Generation: generation,
		},
	}
}
