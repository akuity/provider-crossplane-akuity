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

	"github.com/google/go-cmp/cmp"

	utilcmp "github.com/akuityio/provider-crossplane-akuity/internal/utils/cmp"
)

// DriftSpec describes how one managed resource decides whether its
// desired state matches the observed state.
//
// cmp.Equal is run by default with utilcmp.EquateEmpty() applied
// unconditionally (nil and empty slice/map are equal; zero-valued
// sub-trees are ignored; equivalent CPU/memory quantities are equal).
// Any Ignore options the controller supplies are appended to that
// baseline.
//
// Normalize runs before cmp.Equal. It may mutate desired in place to
// late-init unset fields from observed, strip server-managed keys,
// canonicalise YAML ordering, etc. Controllers that previously called
// a local `normalize*Parameters` helper should move the body here.
//
// Side checks run only if the struct comparison reports equal. Each
// returns (upToDate, error); ALL must return true for the resource to
// be considered up-to-date. Used by KargoInstance for Export-subset
// checks against additively-owned children, Secret/ConfigMap hash
// rotation, and periodic repo-credential re-apply windows.
type DriftSpec[T any] struct {
	Ignore    []cmp.Option
	Normalize func(desired *T, observed *T)
	Side      []func(ctx context.Context) (bool, error)
}

// UpToDate applies Normalize, runs cmp.Equal with the merged options,
// then runs Side. Callers reconcile in this order: Observe fetches
// observed state, then calls UpToDate to decide whether to Update.
//
// Both arguments are pointers so Normalize can mutate. Callers that
// hold a value-type should take its address.
func (d DriftSpec[T]) UpToDate(ctx context.Context, desired, observed *T) (bool, error) {
	if d.Normalize != nil {
		d.Normalize(desired, observed)
	}

	opts := utilcmp.EquateEmpty()
	opts = append(opts, d.Ignore...)
	if !cmp.Equal(*desired, *observed, opts...) {
		return false, nil
	}

	for _, side := range d.Side {
		ok, err := side(ctx)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// Diff returns a human-readable diff between desired and observed,
// using the same options UpToDate applied. Intended for Debug logging
// when drift is detected.
func (d DriftSpec[T]) Diff(desired, observed *T) string {
	opts := utilcmp.EquateEmpty()
	opts = append(opts, d.Ignore...)
	return cmp.Diff(*desired, *observed, opts...)
}
