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
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// SetHealthCondition writes Available or Unavailable based on the
// health flag. Controllers derive health from the Akuity API's
// HealthStatus.Code (== 1 is healthy, see observation.*.HealthStatus)
// and pass the boolean. Consolidates the if/else branch every
// controller's Observe otherwise repeats.
func SetHealthCondition(mg resource.LegacyManaged, healthy bool) { //nolint:staticcheck // cluster-scoped MRs are intentional
	if healthy {
		mg.SetConditions(xpv1.Available())
		return
	}
	mg.SetConditions(xpv1.Unavailable())
}
