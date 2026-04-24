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

package base

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyParams struct {
	Name string
}

func TestEvaluateDrift_UpToDate(t *testing.T) {
	spec := DriftSpec[dummyParams]{}
	d := dummyParams{Name: "same"}
	o := dummyParams{Name: "same"}
	upToDate, err := EvaluateDrift(context.Background(), spec, &d, &o, logging.NewNopLogger(), "test")
	require.NoError(t, err)
	assert.True(t, upToDate)
}

func TestEvaluateDrift_NotUpToDate(t *testing.T) {
	spec := DriftSpec[dummyParams]{}
	d := dummyParams{Name: "desired"}
	o := dummyParams{Name: "observed"}
	upToDate, err := EvaluateDrift(context.Background(), spec, &d, &o, logging.NewNopLogger(), "test")
	require.NoError(t, err)
	assert.False(t, upToDate)
}

// TestEvaluateDrift_SideErrorPropagates guards against EvaluateDrift
// swallowing a side-check failure, which would silently mark a truly
// drifted resource up-to-date.
func TestEvaluateDrift_SideErrorPropagates(t *testing.T) {
	spec := DriftSpec[dummyParams]{
		Side: []func(ctx context.Context) (bool, error){
			func(ctx context.Context) (bool, error) { return false, errors.New("side boom") },
		},
	}
	d := dummyParams{Name: "same"}
	o := dummyParams{Name: "same"}
	_, err := EvaluateDrift(context.Background(), spec, &d, &o, logging.NewNopLogger(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "side boom")
}
