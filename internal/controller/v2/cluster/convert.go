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

package cluster

import (
	"encoding/json"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
)

// specToAPI materialises the Akuity wire Cluster from the curated v2
// spec. The sub-tree of ClusterData uses the WS-3 generated converter;
// wrapping TypeMeta/ObjectMeta + top-level fields is hand-written
// because codegen skips the wrapper (no akuity-side Cluster type in
// the walker scope).
func specToAPI(in v1alpha2.ClusterParameters) akuitytypes.Cluster {
	if in.Data.Size == "" {
		in.Data.Size = v1alpha2.ClusterSize("small")
	}

	var data akuitytypes.ClusterData
	if d := convert.ClusterDataSpecToAPI(&in.Data); d != nil {
		data = *d
	}

	return akuitytypes.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "argocd.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        in.Name,
			Namespace:   in.Namespace,
			Labels:      in.Labels,
			Annotations: in.Annotations,
		},
		Spec: akuitytypes.ClusterSpec{
			Description:     in.Description,
			NamespaceScoped: in.NamespaceScoped,
			Data:            data,
		},
	}
}

// apiToSpec folds the Akuity API cluster payload back into the curated
// v2 ClusterParameters. Fields that the user controls locally (InstanceRef,
// InstanceID) are carried over from the managed resource so drift
// detection compares apples to apples.
func apiToSpec(instanceID string, desired v1alpha2.ClusterParameters, cl *argocdv1.Cluster) v1alpha2.ClusterParameters {
	data := clusterDataFromPB(cl.GetData())

	out := v1alpha2.ClusterParameters{
		InstanceID:      instanceID,
		InstanceRef:     desired.InstanceRef,
		Name:            cl.GetName(),
		Namespace:       cl.GetData().GetNamespace(),
		Description:     cl.GetDescription(),
		NamespaceScoped: cl.GetData().GetNamespaceScoped(),
		Labels:          cl.GetData().GetLabels(),
		Annotations:     cl.GetData().GetAnnotations(),
		Data:            data,
	}
	return out
}

// apiToObservation builds the AtProvider block. Unlike apiToSpec this
// is a simple projection — we copy what the Akuity API reports
// verbatim.
func apiToObservation(cl *argocdv1.Cluster) v1alpha2.ClusterObservation {
	obs := v1alpha2.ClusterObservation{
		ID:                  cl.GetId(),
		Name:                cl.GetName(),
		Description:         cl.GetDescription(),
		Namespace:           cl.GetData().GetNamespace(),
		NamespaceScoped:     cl.GetData().GetNamespaceScoped(),
		Labels:              cl.GetData().GetLabels(),
		Annotations:         cl.GetData().GetAnnotations(),
		AutoUpgradeDisabled: cl.GetData().GetAutoUpgradeDisabled(),
		AppReplication:      cl.GetData().GetAppReplication(),
		TargetVersion:       cl.GetData().GetTargetVersion(),
		RedisTunneling:      cl.GetData().GetRedisTunneling(),
		AgentSize:           clusterSizeString(cl.GetData().GetSize()),
		Kustomization:       pbStructToYAML(cl.GetData().GetKustomization()),
	}
	if h := cl.GetHealthStatus(); h != nil {
		obs.HealthStatus = v1alpha2.ResourceStatusCode{Code: int32(h.GetCode()), Message: h.GetMessage()}
	}
	if r := cl.GetReconciliationStatus(); r != nil {
		obs.ReconciliationStatus = v1alpha2.ResourceStatusCode{Code: int32(r.GetCode()), Message: r.GetMessage()}
	}
	return obs
}

// clusterDataFromPB translates the protobuf ClusterData into the v2
// spec shape. Pointer-valued booleans in the spec model the distinction
// between "unset" and "false" to preserve server-side defaults during
// drift detection.
func clusterDataFromPB(d *argocdv1.ClusterData) v1alpha2.ClusterData {
	if d == nil {
		return v1alpha2.ClusterData{}
	}
	out := v1alpha2.ClusterData{
		Size:                v1alpha2.ClusterSize(clusterSizeString(d.GetSize())),
		AutoUpgradeDisabled: ptr.To(d.GetAutoUpgradeDisabled()),
		Kustomization:       pbStructToYAML(d.GetKustomization()),
		AppReplication:      ptr.To(d.GetAppReplication()),
		TargetVersion:       d.GetTargetVersion(),
		RedisTunneling:      ptr.To(d.GetRedisTunneling()),
		Project:             d.GetProject(),
	}
	return out
}

// pbStructToYAML renders a structpb.Struct (the Akuity API's on-wire
// representation for free-form Kustomization payloads) back to the
// YAML string the v1alpha2 spec exposes. Decode failures yield empty
// string to keep the drift-detection path total — the reconcile-level
// Apply call is the authoritative error surface for malformed
// Kustomization content.
func pbStructToYAML(s *structpb.Struct) string {
	if s == nil || len(s.GetFields()) == 0 {
		return ""
	}
	jsonBytes, err := json.Marshal(s.AsMap())
	if err != nil {
		return ""
	}
	y, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return ""
	}
	return string(y)
}

func clusterSizeString(s argocdv1.ClusterSize) string {
	switch s {
	case argocdv1.ClusterSize_CLUSTER_SIZE_MEDIUM:
		return "medium"
	case argocdv1.ClusterSize_CLUSTER_SIZE_LARGE:
		return "large"
	case argocdv1.ClusterSize_CLUSTER_SIZE_AUTO:
		return "auto"
	default:
		return "small"
	}
}
