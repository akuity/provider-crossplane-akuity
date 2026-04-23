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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type driftSample struct {
	Name    string
	Items   []string
	Ignored string
}

func TestDriftSpec_NilVsEmptySlice_UpToDate(t *testing.T) {
	// Core reviewer bug: user writes `items: []`, API returns nothing
	// (nil). Plain reflect.DeepEqual flags this as drift. DriftSpec
	// applies EquateEmpty so it doesn't.
	desired := driftSample{Items: []string{}}
	observed := driftSample{Items: nil}

	ok, err := DriftSpec[driftSample]{}.UpToDate(context.Background(), &desired, &observed)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestDriftSpec_IgnoreFields(t *testing.T) {
	desired := driftSample{Name: "a", Ignored: "x"}
	observed := driftSample{Name: "a", Ignored: "y"}

	spec := DriftSpec[driftSample]{
		Ignore: []cmp.Option{cmpopts.IgnoreFields(driftSample{}, "Ignored")},
	}

	ok, err := spec.UpToDate(context.Background(), &desired, &observed)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestDriftSpec_NormalizeRunsBeforeEqual(t *testing.T) {
	// Normalize late-inits Name on desired from observed; after
	// normalization the two match.
	desired := driftSample{}
	observed := driftSample{Name: "from-server"}

	spec := DriftSpec[driftSample]{
		Normalize: func(d, o *driftSample) {
			if d.Name == "" {
				d.Name = o.Name
			}
		},
	}

	ok, err := spec.UpToDate(context.Background(), &desired, &observed)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "from-server", desired.Name)
}

func TestDriftSpec_SideCheckFalseBlocksUpToDate(t *testing.T) {
	desired := driftSample{Name: "a"}
	observed := driftSample{Name: "a"}

	spec := DriftSpec[driftSample]{
		Side: []func(ctx context.Context) (bool, error){
			func(_ context.Context) (bool, error) { return false, nil },
		},
	}

	ok, err := spec.UpToDate(context.Background(), &desired, &observed)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestDriftSpec_SideCheckSkippedOnStructDrift(t *testing.T) {
	// Struct already disagrees; Side must not run.
	desired := driftSample{Name: "a"}
	observed := driftSample{Name: "b"}

	sideRan := false
	spec := DriftSpec[driftSample]{
		Side: []func(ctx context.Context) (bool, error){
			func(_ context.Context) (bool, error) { sideRan = true; return true, nil },
		},
	}

	ok, err := spec.UpToDate(context.Background(), &desired, &observed)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.False(t, sideRan, "Side must not run when struct comparison already fails")
}

func TestDriftSpec_SideErrorPropagates(t *testing.T) {
	desired := driftSample{Name: "a"}
	observed := driftSample{Name: "a"}

	want := errors.New("boom")
	spec := DriftSpec[driftSample]{
		Side: []func(ctx context.Context) (bool, error){
			func(_ context.Context) (bool, error) { return false, want },
		},
	}

	ok, err := spec.UpToDate(context.Background(), &desired, &observed)
	assert.False(t, ok)
	require.ErrorIs(t, err, want)
}
