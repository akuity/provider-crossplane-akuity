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

	"k8s.io/utils/ptr"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
)

// specToAPI materialises the Akuity wire Cluster from the curated v2
// spec. The sub-tree of ClusterData uses the generated converter;
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
			NamespaceScoped: ptr.To(in.NamespaceScoped),
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
// spec shape. The heavy lifting — every nested struct and the full
// field set — is delegated to the generated ClusterDataAPIToSpec
// converter; this function only bridges the proto wire to the upstream
// Go wire type the generated code expects. Optional-bool presence is
// preserved by reading the proto pointer fields directly rather than
// through the flattening Get*() accessors.
func clusterDataFromPB(d *argocdv1.ClusterData) v1alpha2.ClusterData {
	if d == nil {
		return v1alpha2.ClusterData{}
	}
	spec := convert.ClusterDataAPIToSpec(pbToAkuityClusterData(d))
	if spec == nil {
		return v1alpha2.ClusterData{}
	}
	return *spec
}

// pbToAkuityClusterData projects the proto ClusterData onto the upstream
// Akuity Go wire type. Optional fields that the proto models as *bool /
// message pointers are copied without dereference so the generated
// converter can distinguish "absent" from "explicit false/empty".
func pbToAkuityClusterData(d *argocdv1.ClusterData) *akuitytypes.ClusterData {
	if d == nil {
		return nil
	}
	out := &akuitytypes.ClusterData{
		Size:                            akuitytypes.ClusterSize(clusterSizeString(d.GetSize())),
		AutoUpgradeDisabled:             d.AutoUpgradeDisabled,
		Kustomization:                   pbStructToRawExtension(d.GetKustomization()),
		AppReplication:                  d.AppReplication,
		TargetVersion:                   d.GetTargetVersion(),
		RedisTunneling:                  d.RedisTunneling,
		DatadogAnnotationsEnabled:       d.DatadogAnnotationsEnabled,
		EksAddonEnabled:                 d.EksAddonEnabled,
		MaintenanceMode:                 d.MaintenanceMode,
		MultiClusterK8SDashboardEnabled: d.MultiClusterK8SDashboardEnabled,
		ServerSideDiffEnabled:           d.ServerSideDiffEnabled,
		MaintenanceModeExpiry:           timestampPBToMetav1(d.GetMaintenanceModeExpiry()),
		Project:                         d.GetProject(),
		// PodInheritMetadata exists on the upstream Go wire type and
		// v1alpha2 spec but is not yet on the proto — leave zero in the
		// projection so drift is only compared where the API can
		// round-trip the value.
	}
	if ds := d.GetDirectClusterSpec(); ds != nil {
		out.DirectClusterSpec = &akuitytypes.DirectClusterSpec{
			ClusterType:     akuitytypes.DirectClusterType(ds.GetClusterType().String()),
			KargoInstanceId: strPtrFromPB(ds.KargoInstanceId),
			Server:          strPtrFromPB(ds.Server),
			Organization:    strPtrFromPB(ds.Organization),
			Token:           strPtrFromPB(ds.Token),
			CaData:          strPtrFromPB(ds.CaData),
		}
	}
	if mc := d.GetManagedClusterConfig(); mc != nil {
		out.ManagedClusterConfig = &akuitytypes.ManagedClusterConfig{
			SecretName: mc.GetSecretName(),
			SecretKey:  mc.GetSecretKey(),
		}
	}
	if asc := d.GetAutoscalerConfig(); asc != nil {
		out.AutoscalerConfig = pbToAutoscalerConfig(asc)
	}
	if c := d.GetCompatibility(); c != nil {
		out.Compatibility = &akuitytypes.ClusterCompatibility{Ipv6Only: ptr.To(c.GetIpv6Only())}
	}
	if n := d.GetArgocdNotificationsSettings(); n != nil {
		out.ArgocdNotificationsSettings = &akuitytypes.ClusterArgoCDNotificationsSettings{
			InClusterSettings: ptr.To(n.GetInClusterSettings()),
		}
	}
	return out
}

func pbToAutoscalerConfig(asc *argocdv1.AutoScalerConfig) *akuitytypes.AutoScalerConfig {
	if asc == nil {
		return nil
	}
	out := &akuitytypes.AutoScalerConfig{}
	if a := asc.GetApplicationController(); a != nil {
		out.ApplicationController = &akuitytypes.AppControllerAutoScalingConfig{
			ResourceMinimum: pbResources(a.GetResourceMinimum()),
			ResourceMaximum: pbResources(a.GetResourceMaximum()),
		}
	}
	if r := asc.GetRepoServer(); r != nil {
		out.RepoServer = &akuitytypes.RepoServerAutoScalingConfig{
			ResourceMinimum: pbResources(r.GetResourceMinimum()),
			ResourceMaximum: pbResources(r.GetResourceMaximum()),
			ReplicaMaximum:  r.GetReplicaMaximum(),
			ReplicaMinimum:  r.GetReplicaMinimum(),
		}
	}
	return out
}

func pbResources(r *argocdv1.Resources) *akuitytypes.Resources {
	if r == nil {
		return nil
	}
	return &akuitytypes.Resources{Mem: r.GetMem(), Cpu: r.GetCpu()}
}

func strPtrFromPB(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

// timestampPBToMetav1 maps an optional protobuf timestamp onto the
// metav1.Time pointer shape used on the akuity Go wire type. Nil
// preserves presence semantics through the generated converter.
func timestampPBToMetav1(t *timestamppb.Timestamp) *metav1.Time {
	if t == nil {
		return nil
	}
	mt := metav1.NewTime(t.AsTime())
	return &mt
}

// pbStructToRawExtension renders a protobuf Struct into the
// runtime.RawExtension shape that akuitytypes.ClusterData.Kustomization
// exposes. The downstream generated converter then formats it as a YAML
// string for the v1alpha2 spec.
func pbStructToRawExtension(s *structpb.Struct) runtime.RawExtension {
	if s == nil || len(s.GetFields()) == 0 {
		return runtime.RawExtension{}
	}
	b, err := json.Marshal(s.AsMap())
	if err != nil {
		return runtime.RawExtension{}
	}
	return runtime.RawExtension{Raw: b}
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
	case argocdv1.ClusterSize_CLUSTER_SIZE_UNSPECIFIED, argocdv1.ClusterSize_CLUSTER_SIZE_SMALL:
		return "small"
	}
	return "small"
}
