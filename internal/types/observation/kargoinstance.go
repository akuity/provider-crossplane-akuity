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

// Package observation hosts AtProvider builders: pure functions that
// project the Akuity gateway response into the ObservationState slot
// of a managed resource. No reconcile state, no side effects — safe
// to call from Observe paths, from drift-detection helpers, and from
// tests without the controller harness.
package observation

import (
	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
)

// KargoInstance projects the Kargo-plane response into the
// KargoInstance AtProvider block.
func KargoInstance(ki *kargov1.KargoInstance) v1alpha1.KargoInstanceObservation {
	if ki == nil {
		return v1alpha1.KargoInstanceObservation{}
	}
	obs := v1alpha1.KargoInstanceObservation{
		ID:                    ki.GetId(),
		Name:                  ki.GetName(),
		Hostname:              ki.GetHostname(),
		OwnerOrganizationName: ki.GetOwnerOrganizationName(),
	}
	if h := ki.GetHealthStatus(); h != nil {
		obs.HealthStatus = v1alpha1.ResourceStatusCode{Code: int32(h.GetCode()), Message: h.GetMessage()}
	}
	if r := ki.GetReconciliationStatus(); r != nil {
		obs.ReconciliationStatus = v1alpha1.ResourceStatusCode{Code: int32(r.GetCode()), Message: r.GetMessage()}
	}
	return obs
}
