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

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

// EvaluateDrift runs spec.UpToDate(ctx, desired, observed) and logs a
// spec.Diff to Debug when drift is present. Caller owns the DeepCopy
// of desired/observed before calling. Normalize may mutate, so shared
// slice/map backing must not leak back into mg. The resource label is
// included on the debug log so operators filtering by controller can
// spot drift without cross-referencing the log context.
func EvaluateDrift[T any](
	ctx context.Context,
	spec DriftSpec[T],
	desired, observed *T,
	logger logging.Logger,
	resource string,
) (bool, error) {
	upToDate, err := spec.UpToDate(ctx, desired, observed)
	if err != nil {
		return false, err
	}
	if !upToDate {
		logger.Debug("drift detected", "resource", resource, "diff", spec.Diff(desired, observed))
	}
	return upToDate, nil
}
