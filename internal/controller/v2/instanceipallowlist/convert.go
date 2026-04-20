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

package instanceipallowlist

import (
	"fmt"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
)

// argoCDPBFromProto re-wraps a (mutated) protobuf Instance.Spec into
// the Argocd payload ApplyInstance expects. We round-trip via the Go
// akuity-types model because structpb.NewStruct doesn't accept
// protobuf messages directly.
func argoCDPBFromProto(name string, ai *argocdv1.Instance) (*structpb.Struct, error) {
	// protojson-encode just the InstanceSpec sub-tree, decode into the
	// hand-authored akuity wire type used by downstream converters
	// (same bridge the Instance controller uses).
	m, err := marshal.ProtoToMap(ai.GetSpec())
	if err != nil {
		return nil, fmt.Errorf("encode InstanceSpec: %w", err)
	}
	wire := akuitytypes.InstanceSpec{}
	if err := marshal.RemarshalTo(m, &wire); err != nil {
		return nil, fmt.Errorf("decode InstanceSpec: %w", err)
	}

	argocd := akuitytypes.ArgoCD{
		TypeMeta:   metav1.TypeMeta{Kind: "ArgoCD", APIVersion: "argocd.akuity.io/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: akuitytypes.ArgoCDSpec{
			Description:  ai.GetDescription(),
			Version:      ai.GetVersion(),
			Shard:        ai.GetShard(),
			InstanceSpec: wire,
		},
	}

	// Round-trip through JSON → map → structpb so nested field
	// names honor the JSON tags on akuity types (matches what the
	// Instance controller's builder produces).
	return marshal.APIModelToPBStruct(argocd)
}
