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
	"fmt"
	"strings"
	"time"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	generated "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// BuildApplyInstanceRequest returns an ApplyInstanceRequest that
// narrow-merges only the Clusters slice (with this one cluster) into
// the target Instance. Sibling fields (Argocd envelope, ArgocdConfigmap,
// Applications, AppProjects, etc.) are left untouched by the server.
// OrganizationId is filled in by the akuity client wrapper.
func BuildApplyInstanceRequest(instanceID string, cluster v1alpha1.ClusterParameters) (*argocdv1.ApplyInstanceRequest, error) {
	wire, err := SpecToAPI(cluster)
	if err != nil {
		return nil, fmt.Errorf("could not build apply cluster wire form: %w", err)
	}
	clusterPB, err := marshal.APIModelToPBStruct(wire)
	if err != nil {
		return nil, fmt.Errorf("could not marshal cluster %s to protobuf struct: %w", cluster.Name, err)
	}
	return &argocdv1.ApplyInstanceRequest{
		IdType:   idv1.Type_ID,
		Id:       instanceID,
		Clusters: []*structpb.Struct{clusterPB},
	}, nil
}

// APIToSpec rebuilds ClusterParameters from the argocd-plane
// response. MR-local fields (InstanceRef, KubeConfigSecretRef,
// EnableInClusterKubeConfig, RemoveAgentResourcesOnDestroy) the
// Akuity API does not own are carried from the managed resource.
func APIToSpec(instanceID string, managedCluster v1alpha1.ClusterParameters, cluster *argocdv1.Cluster) (v1alpha1.ClusterParameters, error) {
	kustomizationYAML, err := marshal.PBStructToKustomizationYAML(cluster.GetData().GetKustomization())
	if err != nil {
		return v1alpha1.ClusterParameters{}, err
	}

	labels := cluster.GetData().GetLabels()
	if len(labels) == 0 {
		labels = nil
	}
	annotations := cluster.GetData().GetAnnotations()
	if len(annotations) == 0 {
		annotations = nil
	}

	size := "small"
	switch cluster.GetData().GetSize() { //nolint:exhaustive
	case argocdv1.ClusterSize_CLUSTER_SIZE_MEDIUM:
		size = "medium"
	case argocdv1.ClusterSize_CLUSTER_SIZE_LARGE:
		size = "large"
	case argocdv1.ClusterSize_CLUSTER_SIZE_AUTO:
		size = "auto"
	}

	return v1alpha1.ClusterParameters{
		InstanceID:  instanceID,
		InstanceRef: managedCluster.InstanceRef,
		Name:        cluster.GetName(),
		Namespace:   cluster.GetData().GetNamespace(),
		Labels:      labels,
		Annotations: annotations,
		ClusterSpec: generated.ClusterSpec{
			Description:     cluster.GetDescription(),
			NamespaceScoped: ptr.To(cluster.GetData().GetNamespaceScoped()),
			Data: generated.ClusterData{
				Size:                            generated.ClusterSize(size),
				AutoUpgradeDisabled:             ptr.To(cluster.GetData().GetAutoUpgradeDisabled()),
				Kustomization:                   string(kustomizationYAML),
				AppReplication:                  ptr.To(cluster.GetData().GetAppReplication()),
				TargetVersion:                   cluster.GetData().GetTargetVersion(),
				RedisTunneling:                  ptr.To(cluster.GetData().GetRedisTunneling()),
				DirectClusterSpec:               apiToDirectClusterSpec(cluster.GetData().GetDirectClusterSpec()),
				DatadogAnnotationsEnabled:       cluster.GetData().DatadogAnnotationsEnabled, //nolint:all
				EksAddonEnabled:                 cluster.GetData().EksAddonEnabled,           //nolint:all
				ManagedClusterConfig:            apiToManagedClusterConfig(cluster.GetData().GetManagedClusterConfig()),
				MultiClusterK8SDashboardEnabled: cluster.GetData().MultiClusterK8SDashboardEnabled, //nolint:all
				AutoscalerConfig:                apiToAutoscalerConfig(cluster.GetData().GetAutoscalerConfig()),
				Project:                         cluster.GetData().GetProject(),
				Compatibility:                   apiToCompatibility(cluster.GetData().GetCompatibility()),
				ArgocdNotificationsSettings:     apiToArgocdNotificationsSettings(cluster.GetData().GetArgocdNotificationsSettings()),
				ServerSideDiffEnabled:           cluster.GetData().ServerSideDiffEnabled, //nolint:all
				// MaintenanceMode + MaintenanceModeExpiry are server-owned
				// for drift purposes: ApplyInstance does not propagate
				// them because they ride a dedicated set-maintenance-mode RPC,
				// and ExportInstance therefore does not echo them either.
				// GetCluster is the only response shape that carries the
				// canonical values, so the drift comparator targets the
				// Get-derived spec for these specific fields (see Observe
				// in cluster.go for the override that lifts these values
				// onto the Export-derived driftTarget).
				MaintenanceMode:       cluster.GetData().MaintenanceMode, //nolint:all
				MaintenanceModeExpiry: timestampPtrToStringPtr(cluster.GetData().GetMaintenanceModeExpiry()),
				PodInheritMetadata:    cluster.GetData().PodInheritMetadata, //nolint:all
			},
		},
		EnableInClusterKubeConfig: managedCluster.EnableInClusterKubeConfig,
		KubeConfigSecretRef: xpv1.SecretReference{
			Name:      managedCluster.KubeConfigSecretRef.Name,
			Namespace: managedCluster.KubeConfigSecretRef.Namespace,
		},
		RemoveAgentResourcesOnDestroy: managedCluster.RemoveAgentResourcesOnDestroy,
	}, nil
}

// wireToSpec rebuilds ClusterParameters from the Akuity wire-form
// Cluster that ExportInstance returns in its Clusters slice. The
// struct round-trips through the generated ClusterDataAPIToSpec
// converter (same one used by the Apply path), giving drift detection
// a canonical shape that matches what the Apply request sends.
// InstanceID and MR-local fields are copied from managedCluster; the
// Akuity API does not own them.
func wireToSpec(instanceID string, managedCluster v1alpha1.ClusterParameters, wireCluster *akuitytypes.Cluster) v1alpha1.ClusterParameters {
	if wireCluster == nil {
		return v1alpha1.ClusterParameters{}
	}
	out := v1alpha1.ClusterParameters{
		InstanceID:  instanceID,
		InstanceRef: managedCluster.InstanceRef,
		Name:        wireCluster.GetName(),
		Namespace:   wireCluster.Namespace,
		Labels:      wireCluster.Labels,
		Annotations: wireCluster.Annotations,
		ClusterSpec: generated.ClusterSpec{
			Description:     wireCluster.Spec.Description,
			NamespaceScoped: wireCluster.Spec.NamespaceScoped,
		},
		EnableInClusterKubeConfig: managedCluster.EnableInClusterKubeConfig,
		KubeConfigSecretRef: xpv1.SecretReference{
			Name:      managedCluster.KubeConfigSecretRef.Name,
			Namespace: managedCluster.KubeConfigSecretRef.Namespace,
		},
		RemoveAgentResourcesOnDestroy: managedCluster.RemoveAgentResourcesOnDestroy,
	}
	if data := generated.ClusterDataAPIToSpec(&wireCluster.Spec.Data); data != nil {
		out.ClusterSpec.Data = *data
	}
	// Normalise server-reported empty collections to nil so downstream
	// cmp.Equal + EquateEmpty comparisons treat them the same as an
	// unset user spec.
	if len(out.Labels) == 0 {
		out.Labels = nil
	}
	if len(out.Annotations) == 0 {
		out.Annotations = nil
	}
	return out
}

// SpecToAPI produces the wire-form Cluster that buildApplyClusterRequest
// marshals into the ApplyInstance payload.
func SpecToAPI(cluster v1alpha1.ClusterParameters) (akuitytypes.Cluster, error) {
	if cluster.ClusterSpec.Data.Size == "" {
		cluster.ClusterSpec.Data.Size = "small"
	}
	if err := projectCustomClusterSize(&cluster.ClusterSpec.Data); err != nil {
		return akuitytypes.Cluster{}, err
	}

	kustomization, err := clusterKustomizationRaw(cluster.ClusterSpec.Data.Kustomization)
	if err != nil {
		return akuitytypes.Cluster{}, fmt.Errorf("spec.forProvider.clusterSpec.data.kustomization: %w", err)
	}
	data := generated.ClusterDataSpecToAPI(&cluster.ClusterSpec.Data)
	if data == nil {
		data = &akuitytypes.ClusterData{}
	}
	data.Kustomization = kustomization
	// MaintenanceMode and MaintenanceModeExpiry are owned by the
	// dedicated set-maintenance-mode RPC; never send them through
	// ApplyInstance even if the generated converter can populate them.
	data.MaintenanceMode = nil
	data.MaintenanceModeExpiry = nil

	return akuitytypes.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "argocd.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name,
			Namespace:   cluster.Namespace,
			Labels:      cluster.Labels,
			Annotations: cluster.Annotations,
		},
		Spec: akuitytypes.ClusterSpec{
			Description:     cluster.ClusterSpec.Description,
			NamespaceScoped: cluster.ClusterSpec.NamespaceScoped,
			Data:            *data,
		},
	}, nil
}

func clusterKustomizationRaw(s string) (runtime.RawExtension, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || trimmed == "null" {
		return runtime.RawExtension{}, nil
	}
	raw, err := yaml.YAMLToJSON([]byte(s))
	if err != nil {
		return runtime.RawExtension{}, fmt.Errorf("invalid kustomization YAML: %w", err)
	}
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return runtime.RawExtension{}, fmt.Errorf("kustomization YAML must be an object at the top level: %w", err)
	}
	if top == nil {
		return runtime.RawExtension{}, nil
	}
	if err := validateClusterKustomizationObject(top); err != nil {
		return runtime.RawExtension{}, err
	}
	if _, ok := top["apiVersion"]; !ok {
		top["apiVersion"] = "kustomize.config.k8s.io/v1beta1"
	}
	if _, ok := top["kind"]; !ok {
		top["kind"] = "Kustomization"
	}
	raw, err = json.Marshal(top)
	if err != nil {
		return runtime.RawExtension{}, fmt.Errorf("could not encode kustomization JSON: %w", err)
	}
	return runtime.RawExtension{Raw: raw}, nil
}

func validateClusterKustomizationObject(top map[string]any) error {
	for k, v := range top {
		if !clusterKustomizationTopLevelKeys[k] {
			return fmt.Errorf("unknown top-level Kustomization field %q", k)
		}
		if k == "kind" && v != "Kustomization" {
			return fmt.Errorf("kind must be Kustomization when set")
		}
		if (k == "apiVersion" || k == "kind") && v != nil {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s must be a string when set", k)
			}
		}
	}
	return nil
}

var clusterKustomizationTopLevelKeys = map[string]bool{
	"apiVersion":                true,
	"kind":                      true,
	"metadata":                  true,
	"namespace":                 true,
	"namePrefix":                true,
	"nameSuffix":                true,
	"commonLabels":              true,
	"labels":                    true,
	"commonAnnotations":         true,
	"resources":                 true,
	"bases":                     true,
	"components":                true,
	"crds":                      true,
	"configurations":            true,
	"vars":                      true,
	"images":                    true,
	"replicas":                  true,
	"patches":                   true,
	"patchesJson6902":           true,
	"patchesStrategicMerge":     true,
	"replacements":              true,
	"configMapGenerator":        true,
	"secretGenerator":           true,
	"generators":                true,
	"generatorOptions":          true,
	"transformers":              true,
	"validators":                true,
	"openapi":                   true,
	"buildMetadata":             true,
	"helmCharts":                true,
	"helmGlobals":               true,
	"sortOptions":               true,
	"configMapGeneratorOptions": true,
	"secretGeneratorOptions":    true,
}

func apiToDirectClusterSpec(in *argocdv1.DirectClusterSpec) *generated.DirectClusterSpec {
	if in == nil {
		return nil
	}
	return &generated.DirectClusterSpec{
		ClusterType:     apiToDirectClusterType(in.GetClusterType()),
		KargoInstanceId: in.KargoInstanceId,
		Server:          in.Server,
		Organization:    in.Organization,
		Token:           in.Token,
		CaData:          in.CaData,
	}
}

func apiToDirectClusterType(in argocdv1.DirectClusterType) generated.DirectClusterType {
	switch in {
	case argocdv1.DirectClusterType_DIRECT_CLUSTER_TYPE_KARGO:
		return "kargo"
	case argocdv1.DirectClusterType_DIRECT_CLUSTER_TYPE_UPBOUND:
		return "upbound"
	default:
		return ""
	}
}

func apiToManagedClusterConfig(in *argocdv1.ManagedClusterConfig) *generated.ManagedClusterConfig {
	if in == nil {
		return nil
	}
	return &generated.ManagedClusterConfig{
		SecretName: in.GetSecretName(),
		SecretKey:  in.GetSecretKey(),
	}
}

func apiToAutoscalerConfig(in *argocdv1.AutoScalerConfig) *generated.AutoScalerConfig {
	if in == nil {
		return nil
	}
	out := &generated.AutoScalerConfig{}
	if in.GetApplicationController() != nil {
		ac := &generated.AppControllerAutoScalingConfig{}
		if in.GetApplicationController().GetResourceMinimum() != nil {
			ac.ResourceMinimum = &generated.Resources{
				Mem: in.GetApplicationController().GetResourceMinimum().GetMem(),
				Cpu: in.GetApplicationController().GetResourceMinimum().GetCpu(),
			}
		}
		if in.GetApplicationController().GetResourceMaximum() != nil {
			ac.ResourceMaximum = &generated.Resources{
				Mem: in.GetApplicationController().GetResourceMaximum().GetMem(),
				Cpu: in.GetApplicationController().GetResourceMaximum().GetCpu(),
			}
		}
		out.ApplicationController = ac
	}
	if in.GetRepoServer() != nil {
		rs := &generated.RepoServerAutoScalingConfig{}
		if in.GetRepoServer().GetResourceMinimum() != nil {
			rs.ResourceMinimum = &generated.Resources{
				Mem: in.GetRepoServer().GetResourceMinimum().GetMem(),
				Cpu: in.GetRepoServer().GetResourceMinimum().GetCpu(),
			}
		}
		if in.GetRepoServer().GetResourceMaximum() != nil {
			rs.ResourceMaximum = &generated.Resources{
				Mem: in.GetRepoServer().GetResourceMaximum().GetMem(),
				Cpu: in.GetRepoServer().GetResourceMaximum().GetCpu(),
			}
		}
		rs.ReplicaMinimum = in.GetRepoServer().GetReplicaMinimum()
		rs.ReplicaMaximum = in.GetRepoServer().GetReplicaMaximum()
		out.RepoServer = rs
	}
	return out
}

// timestampPtrToStringPtr renders the gateway's *timestamppb.Timestamp
// shape into the curated *string (RFC3339) shape used on the Crossplane
// CRD. Nil or zero input yields nil so callers compose without
// pre-checks. Mirrors generated.TimePtrToStringPtr but bridges the
// argocdv1 wire type rather than the akuitytypes intermediate; the
// GetCluster path returns the gateway shape directly without going
// through ClusterDataAPIToSpec.
func timestampPtrToStringPtr(t *timestamppb.Timestamp) *string {
	if t == nil {
		return nil
	}
	tt := t.AsTime()
	if tt.IsZero() {
		return nil
	}
	s := tt.UTC().Format(time.RFC3339)
	return &s
}

func apiToCompatibility(in *argocdv1.ClusterCompatibility) *generated.ClusterCompatibility {
	if in == nil {
		return nil
	}
	return &generated.ClusterCompatibility{
		Ipv6Only: ptr.To(in.GetIpv6Only()),
	}
}

// apiToArgocdNotificationsSettings lifts the GetCluster-shaped
// ArgoCDNotificationsSettings onto the curated spec. Get echoes
// server-stamped defaults that the Export response omits, so this
// projection is the only way the drift comparator can see the
// canonical observed value for these settings (see Observe override
// in cluster.go).
func apiToArgocdNotificationsSettings(in *argocdv1.ClusterArgoCDNotificationsSettings) *generated.ClusterArgoCDNotificationsSettings {
	if in == nil {
		return nil
	}
	return &generated.ClusterArgoCDNotificationsSettings{
		InClusterSettings: ptr.To(in.GetInClusterSettings()),
	}
}
