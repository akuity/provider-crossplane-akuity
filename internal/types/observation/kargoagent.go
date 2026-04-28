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

package observation

import (
	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
)

// KargoAgent projects the Kargo-plane agent response into the
// KargoAgent AtProvider block. Workspace intentionally stays empty
// here because the controller owns it locally and carries it via the
// desired spec.
func KargoAgent(agent *kargov1.KargoAgent) v1alpha1.KargoAgentObservation {
	if agent == nil {
		return v1alpha1.KargoAgentObservation{}
	}
	obs := v1alpha1.KargoAgentObservation{
		ID:   agent.GetId(),
		Name: agent.GetName(),
	}
	if h := agent.GetHealthStatus(); h != nil {
		obs.HealthStatus = v1alpha1.ResourceStatusCode{Code: int32(h.GetCode()), Message: h.GetMessage()}
	}
	if r := agent.GetReconciliationStatus(); r != nil {
		obs.ReconciliationStatus = v1alpha1.ResourceStatusCode{Code: int32(r.GetCode()), Message: r.GetMessage()}
	}
	return obs
}
