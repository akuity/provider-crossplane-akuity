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
	"github.com/akuityio/provider-crossplane-akuity/internal/convert/glue"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
)

// apiToSpec rebuilds the v1alpha2 KargoAgentParameters from the
// observed Akuity KargoAgent. Fields that the user owns locally
// (KargoInstanceID / KargoInstanceRef / Workspace) are carried over
// from the managed resource so drift detection compares apples to
// apples. Namespace / Labels / Annotations live inside the proto Data
// sub-tree on the wire.
func apiToSpec(desired v1alpha2.KargoAgentParameters, agent *kargov1.KargoAgent) v1alpha2.KargoAgentParameters {
	data := agent.GetData()
	out := v1alpha2.KargoAgentParameters{
		KargoInstanceID:  desired.KargoInstanceID,
		KargoInstanceRef: desired.KargoInstanceRef,
		Name:             agent.GetName(),
		Namespace:        data.GetNamespace(),
		Workspace:        desired.Workspace,
		Labels:           data.GetLabels(),
		Annotations:      data.GetAnnotations(),
	}
	// Project the wire Data sub-tree via the generated converter; the
	// top-level KargoAgentSpec wrapper is constructed here because
	// the wire KargoAgent doesn't expose a separate "spec" field.
	spec := &v1alpha2.KargoAgentSpec{Description: agent.GetDescription()}
	if d := convertAgentData(data); d != nil {
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
// protobuf (via marshal.GoModelToProto). Namespace / Labels /
// Annotations live on the proto but the upstream akuity Go wire type
// does not yet carry them, so they are injected onto the proto message
// after the bridge.
func agentDataPB(p v1alpha2.KargoAgentParameters) (*kargov1.KargoAgentData, error) {
	pb := &kargov1.KargoAgentData{}
	if p.Spec != nil {
		if err := glue.ValidateKustomizationYAML(p.Spec.Data.Kustomization); err != nil {
			return nil, fmt.Errorf("spec.forProvider.spec.data.kustomization: %w", err)
		}
		wire := convert.KargoAgentDataSpecToAPI(&p.Spec.Data)
		if wire == nil {
			wire = &akuitytypes.KargoAgentData{}
		}
		if err := marshal.GoModelToProto(wire, pb); err != nil {
			return nil, fmt.Errorf("encode KargoAgentData: %w", err)
		}
	}
	pb.Namespace = p.Namespace
	pb.Labels = p.Labels
	pb.Annotations = p.Annotations
	return pb, nil
}

// convertAgentData folds the Kargo protobuf KargoAgentData into the
// v1alpha2 spec shape. Goes through the marshal bridge + the
// generated converter, the same way the KargoInstance controller does.
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
