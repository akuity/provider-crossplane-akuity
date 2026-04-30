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
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// SpecToAPI builds the akuity wire-form KargoAgent from the managed
// resource's forProvider spec. The wire form is the same shape
// ExportKargoInstance returns inside its Agents slice, giving
// round-trip symmetry between read (Export) and write (Apply).
func SpecToAPI(p v1alpha1.KargoAgentParameters) (akuitytypes.KargoAgent, error) {
	if err := projectCustomKargoAgentSize(p.Name, &p.KargoAgentSpec.Data); err != nil {
		return akuitytypes.KargoAgent{}, err
	}
	if err := crossplanetypes.ValidateKustomizationYAML(p.KargoAgentSpec.Data.Kustomization); err != nil {
		return akuitytypes.KargoAgent{}, fmt.Errorf("spec.forProvider.kargoAgentSpec.data.kustomization: %w", err)
	}
	data := crossplanetypes.KargoAgentDataSpecToAPI(&p.KargoAgentSpec.Data)
	if data == nil {
		data = &akuitytypes.KargoAgentData{}
	}
	return akuitytypes.KargoAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KargoAgent",
			APIVersion: "kargo.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        p.Name,
			Namespace:   p.Namespace,
			Labels:      p.Labels,
			Annotations: p.Annotations,
		},
		Spec: akuitytypes.KargoAgentSpec{
			Description: p.KargoAgentSpec.Description,
			Data:        *data,
		},
	}, nil
}

// BuildApplyKargoInstanceRequest returns an ApplyKargoInstanceRequest
// that narrow-merges only the Agents slice (with this one agent) into
// the target KargoInstance. Sibling fields (Kargo envelope,
// KargoConfigmap, Projects, Warehouses, Stages, RepoCredentials, etc.)
// are left untouched by the server. OrganizationId is filled in by
// the akuity client wrapper.
func BuildApplyKargoInstanceRequest(kargoInstanceID string, p v1alpha1.KargoAgentParameters) (*kargov1.ApplyKargoInstanceRequest, error) {
	wire, err := SpecToAPI(p)
	if err != nil {
		return nil, err
	}
	agentPB, err := marshal.APIModelToPBStruct(wire)
	if err != nil {
		return nil, fmt.Errorf("could not marshal kargo agent %s to protobuf struct: %w", p.Name, err)
	}
	return &kargov1.ApplyKargoInstanceRequest{
		IdType:      idv1.Type_ID,
		Id:          kargoInstanceID,
		WorkspaceId: p.Workspace,
		Agents:      []*structpb.Struct{agentPB},
	}, nil
}

// apiToSpec rebuilds KargoAgentParameters from the
// observed Akuity KargoAgent. Fields that the user owns locally
// (KargoInstanceID / KargoInstanceRef / Workspace, plus the agent-install
// kubeconfig trio that never round-trips through the Akuity gateway:
// KubeConfigSecretRef / EnableInClusterKubeConfig /
// RemoveAgentResourcesOnDestroy) are carried over from the managed
// resource so drift detection compares apples to apples. Namespace /
// Labels / Annotations live inside the proto Data sub-tree on the wire.
func apiToSpec(desired v1alpha1.KargoAgentParameters, agent *kargov1.KargoAgent) v1alpha1.KargoAgentParameters {
	data := agent.GetData()
	out := v1alpha1.KargoAgentParameters{
		KargoInstanceID:               desired.KargoInstanceID,
		KargoInstanceRef:              desired.KargoInstanceRef,
		Name:                          agent.GetName(),
		Namespace:                     data.GetNamespace(),
		Workspace:                     desired.Workspace,
		Labels:                        data.GetLabels(),
		Annotations:                   data.GetAnnotations(),
		KubeConfigSecretRef:           desired.KubeConfigSecretRef,
		EnableInClusterKubeConfig:     desired.EnableInClusterKubeConfig,
		RemoveAgentResourcesOnDestroy: desired.RemoveAgentResourcesOnDestroy,
	}
	// Description + data live under the KargoAgentSpec wrapper,
	// mirroring the Cluster shape where payload lives under
	// clusterSpec.
	out.KargoAgentSpec.Description = agent.GetDescription()
	if d := convertAgentData(data); d != nil {
		out.KargoAgentSpec.Data = *d
	}
	if data != nil && out.KargoAgentSpec.Data.AkuityManaged == nil {
		out.KargoAgentSpec.Data.AkuityManaged = ptr.To(data.GetAkuityManaged())
	}
	return out
}

// wireToSpec rebuilds KargoAgentParameters from the Akuity wire-form
// KargoAgent that ExportKargoInstance returns inside its Agents slice.
// Namespace / Labels / Annotations live on ObjectMeta in the wire
// form (not on Data, as in the proto). Spec-only fields that the Akuity
// API does not own (KargoInstanceID / KargoInstanceRef / Workspace,
// plus the agent-install kubeconfig trio: KubeConfigSecretRef /
// EnableInClusterKubeConfig / RemoveAgentResourcesOnDestroy) are
// carried from desired so drift detection compares apples to apples.
func wireToSpec(desired v1alpha1.KargoAgentParameters, wire *akuitytypes.KargoAgent) v1alpha1.KargoAgentParameters {
	if wire == nil {
		return v1alpha1.KargoAgentParameters{}
	}
	out := v1alpha1.KargoAgentParameters{
		KargoInstanceID:               desired.KargoInstanceID,
		KargoInstanceRef:              desired.KargoInstanceRef,
		Name:                          wire.GetName(),
		Namespace:                     wire.Namespace,
		Workspace:                     desired.Workspace,
		Labels:                        wire.Labels,
		Annotations:                   wire.Annotations,
		KubeConfigSecretRef:           desired.KubeConfigSecretRef,
		EnableInClusterKubeConfig:     desired.EnableInClusterKubeConfig,
		RemoveAgentResourcesOnDestroy: desired.RemoveAgentResourcesOnDestroy,
	}
	out.KargoAgentSpec.Description = wire.Spec.Description
	if d := crossplanetypes.KargoAgentDataAPIToSpec(&wire.Spec.Data); d != nil {
		out.KargoAgentSpec.Data = *d
	}
	if out.KargoAgentSpec.Data.AkuityManaged == nil &&
		desired.KargoAgentSpec.Data.AkuityManaged != nil &&
		!*desired.KargoAgentSpec.Data.AkuityManaged {
		out.KargoAgentSpec.Data.AkuityManaged = ptr.To(false)
	}
	if len(out.Labels) == 0 {
		out.Labels = nil
	}
	if len(out.Annotations) == 0 {
		out.Annotations = nil
	}
	return out
}

// convertAgentData folds the Kargo protobuf KargoAgentData into the
// spec shape via the marshal bridge + the generated converter. Decode
// failures are swallowed to nil because the Observe path can recover
// on the next poll; surfacing an error here would retry-loop on a
// schema drift the user cannot fix.
func convertAgentData(pb *kargov1.KargoAgentData) *crossplanetypes.KargoAgentData {
	if pb == nil {
		return nil
	}
	wire, err := marshal.ProtoToWire[akuitytypes.KargoAgentData](pb)
	if err != nil {
		return nil
	}
	return crossplanetypes.KargoAgentDataAPIToSpec(&wire)
}
