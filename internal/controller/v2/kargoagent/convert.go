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

package kargoagent

import (
	"fmt"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
)

// apiToSpec rebuilds the v1alpha2 KargoAgentParameters from the
// observed Akuity KargoAgent. Fields that the user owns locally
// (KargoInstanceID / KargoInstanceRef) are carried over from the
// managed resource so drift detection compares apples to apples.
func apiToSpec(desired v1alpha2.KargoAgentParameters, agent *kargov1.KargoAgent) v1alpha2.KargoAgentParameters {
	out := v1alpha2.KargoAgentParameters{
		KargoInstanceID:  desired.KargoInstanceID,
		KargoInstanceRef: desired.KargoInstanceRef,
		Name:             agent.GetName(),
		Namespace:        agent.GetNamespace(),
		Workspace:        desired.Workspace,
		Labels:           desired.Labels,
		Annotations:      desired.Annotations,
	}
	// Project the wire Data sub-tree via the WS-3 converter; the
	// top-level KargoAgentSpec wrapper is constructed here because
	// the wire KargoAgent doesn't expose a separate "spec" field.
	spec := &v1alpha2.KargoAgentSpec{Description: agent.GetDescription()}
	if d := convertAgentData(agent.GetData()); d != nil {
		spec.Data = *d
	}
	out.Spec = spec
	return out
}

// apiToObservation produces the AtProvider status block.
func apiToObservation(agent *kargov1.KargoAgent) v1alpha2.KargoAgentObservation {
	obs := v1alpha2.KargoAgentObservation{
		ID:        agent.GetId(),
		Workspace: "",
	}
	if h := agent.GetHealthStatus(); h != nil {
		obs.HealthStatus = v1alpha2.ResourceStatusCode{Code: int32(h.GetCode()), Message: h.GetMessage()}
	}
	if r := agent.GetReconciliationStatus(); r != nil {
		obs.ReconciliationStatus = v1alpha2.ResourceStatusCode{Code: int32(r.GetCode()), Message: r.GetMessage()}
	}
	return obs
}

// agentDataPB materialises the KargoAgentData protobuf payload from
// the v1alpha2 spec. v1alpha2 → akuity wire (via codegen) →
// protobuf (via marshal.GoModelToProto). Unknown fields on the
// protobuf side are silently dropped.
func agentDataPB(p v1alpha2.KargoAgentParameters) (*kargov1.KargoAgentData, error) {
	if p.Spec == nil {
		return &kargov1.KargoAgentData{}, nil
	}
	wire := convert.KargoAgentDataSpecToAPI(&p.Spec.Data)
	if wire == nil {
		wire = &akuitytypes.KargoAgentData{}
	}
	pb := &kargov1.KargoAgentData{}
	if err := marshal.GoModelToProto(wire, pb); err != nil {
		return nil, fmt.Errorf("encode KargoAgentData: %w", err)
	}
	return pb, nil
}

// convertAgentData folds the Kargo protobuf KargoAgentData into the
// v1alpha2 spec shape. Goes through the marshal bridge + WS-3
// generated converter the same way the KargoInstance controller does.
func convertAgentData(pb *kargov1.KargoAgentData) *v1alpha2.KargoAgentData {
	if pb == nil {
		return nil
	}
	m, err := marshal.ProtoToMap(pb)
	if err != nil {
		return nil
	}
	wire := akuitytypes.KargoAgentData{}
	if err := marshal.RemarshalTo(m, &wire); err != nil {
		return nil
	}
	return convert.KargoAgentDataAPIToSpec(&wire)
}
