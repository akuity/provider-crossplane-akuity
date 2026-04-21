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

package instance

import (
	"context"
	"fmt"
	"strconv"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert/glue"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	argocdtypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/argocd/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/utils/protobuf"
)

// Well-known configmap names used in the Akuity export payload.
const (
	argoCDCMKey                = "argocd-cm"
	argoCDImageUpdaterCMKey    = "argocd-image-updater-config"
	argoCDImageUpdaterSSHCMKey = "argocd-image-updater-ssh-config"
	argoCDNotificationsCMKey   = "argocd-notifications-cm"
	argoCDRBACCMKey            = "argocd-rbac-cm"
	argoCDSSHKnownHostsCMKey   = "argocd-ssh-known-hosts-cm"
	argoCDTLSCertsCMKey        = "argocd-tls-certs-cm"
)

// apiToSpec rebuilds the v1alpha2 Instance parameters from a freshly
// observed Akuity API payload. The result is compared field-for-field
// against the managed resource's desired spec to decide up-to-date.
func apiToSpec(ai *argocdv1.Instance, exp *argocdv1.ExportInstanceResponse) (v1alpha2.InstanceParameters, error) {
	spec := v1alpha2.InstanceParameters{
		Name: ai.GetName(),
		ArgoCD: &v1alpha2.ArgoCD{
			Spec: v1alpha2.ArgoCDSpec{
				Description: ai.GetDescription(),
				Version:     ai.GetVersion(),
				Shard:       ai.GetShard(),
			},
		},
	}

	if is, err := pbInstanceSpecToSpec(ai); err != nil {
		return spec, err
	} else if is != nil {
		spec.ArgoCD.Spec.InstanceSpec = *is
	}

	cms := []struct {
		dst *map[string]string
		key string
		pb  *structpb.Struct
	}{
		{&spec.ArgoCDConfigMap, argoCDCMKey, exp.GetArgocdConfigmap()},
		{&spec.ArgoCDImageUpdaterConfigMap, argoCDImageUpdaterCMKey, exp.GetImageUpdaterConfigmap()},
		{&spec.ArgoCDImageUpdaterSSHConfigMap, argoCDImageUpdaterSSHCMKey, exp.GetImageUpdaterSshConfigmap()},
		{&spec.ArgoCDNotificationsConfigMap, argoCDNotificationsCMKey, exp.GetNotificationsConfigmap()},
		{&spec.ArgoCDRBACConfigMap, argoCDRBACCMKey, exp.GetArgocdRbacConfigmap()},
		{&spec.ArgoCDSSHKnownHostsConfigMap, argoCDSSHKnownHostsCMKey, exp.GetArgocdKnownHostsConfigmap()},
		{&spec.ArgoCDTLSCertsConfigMap, argoCDTLSCertsCMKey, exp.GetArgocdTlsCertsConfigmap()},
	}
	for _, cm := range cms {
		v, err := configMapFromPB(cm.key, cm.pb)
		if err != nil {
			return spec, err
		}
		*cm.dst = v
	}

	plugins, err := pluginsFromPB(exp.GetConfigManagementPlugins())
	if err != nil {
		return spec, err
	}
	spec.ConfigManagementPlugins = plugins

	return spec, nil
}

// apiToObservation projects the Akuity API Instance + export payload
// into the observable AtProvider block.
func apiToObservation(ai *argocdv1.Instance, exp *argocdv1.ExportInstanceResponse) v1alpha2.InstanceObservation {
	obs := v1alpha2.InstanceObservation{
		ID:                    ai.GetId(),
		Name:                  ai.GetName(),
		Hostname:              ai.GetHostname(),
		ClusterCount:          ai.GetClusterCount(),
		OwnerOrganizationName: ai.GetOwnerOrganizationName(),
	}

	if is, err := pbInstanceSpecToSpec(ai); err == nil && is != nil {
		obs.ArgoCD = v1alpha2.ArgoCD{
			Spec: v1alpha2.ArgoCDSpec{
				Description:  ai.GetDescription(),
				Version:      ai.GetVersion(),
				Shard:        ai.GetShard(),
				InstanceSpec: *is,
			},
		}
	}

	if h := ai.GetHealthStatus(); h != nil {
		obs.HealthStatus = v1alpha2.ResourceStatusCode{Code: int32(h.GetCode()), Message: h.GetMessage()}
	}
	if r := ai.GetReconciliationStatus(); r != nil {
		obs.ReconciliationStatus = v1alpha2.ResourceStatusCode{Code: int32(r.GetCode()), Message: r.GetMessage()}
	}

	// Status ConfigMaps + plugins are best-effort; decode errors are
	// swallowed to keep the status reporting path non-fatal (the
	// reconcile-level errors surface any wire-format breakage first).
	obs.ArgoCDConfigMap, _ = configMapFromPB(argoCDCMKey, exp.GetArgocdConfigmap())
	obs.ArgoCDImageUpdaterConfigMap, _ = configMapFromPB(argoCDImageUpdaterCMKey, exp.GetImageUpdaterConfigmap())
	obs.ArgoCDImageUpdaterSSHConfigMap, _ = configMapFromPB(argoCDImageUpdaterSSHCMKey, exp.GetImageUpdaterSshConfigmap())
	obs.ArgoCDNotificationsConfigMap, _ = configMapFromPB(argoCDNotificationsCMKey, exp.GetNotificationsConfigmap())
	obs.ArgoCDRBACConfigMap, _ = configMapFromPB(argoCDRBACCMKey, exp.GetArgocdRbacConfigmap())
	obs.ArgoCDSSHKnownHostsConfigMap, _ = configMapFromPB(argoCDSSHKnownHostsCMKey, exp.GetArgocdKnownHostsConfigmap())
	obs.ArgoCDTLSCertsConfigMap, _ = configMapFromPB(argoCDTLSCertsCMKey, exp.GetArgocdTlsCertsConfigmap())
	obs.ConfigManagementPlugins, _ = pluginsFromPB(exp.GetConfigManagementPlugins())

	return obs
}

// lateInitialize fills in defaults from the actual (observed) instance
// into the managed spec for fields the user commonly omits. Keep this
// narrow to match the legacy golden-snapshot expectations. Returns true
// when any field on in was mutated so the caller can signal
// ResourceLateInitialized to the managed.Reconciler.
func lateInitialize(in, actual *v1alpha2.InstanceParameters) bool {
	before := in.DeepCopy()

	if in.ArgoCD != nil && actual.ArgoCD != nil {
		if in.ArgoCD.Spec.InstanceSpec.Subdomain == "" {
			in.ArgoCD.Spec.InstanceSpec.Subdomain = actual.ArgoCD.Spec.InstanceSpec.Subdomain
		}
		if in.ArgoCD.Spec.InstanceSpec.Fqdn == "" {
			in.ArgoCD.Spec.InstanceSpec.Fqdn = actual.ArgoCD.Spec.InstanceSpec.Fqdn
		}
	}

	if in.ArgoCDConfigMap == nil {
		in.ArgoCDConfigMap = actual.ArgoCDConfigMap
	}
	if in.ArgoCDRBACConfigMap == nil {
		in.ArgoCDRBACConfigMap = actual.ArgoCDRBACConfigMap
	}
	if in.ArgoCDSSHKnownHostsConfigMap == nil {
		in.ArgoCDSSHKnownHostsConfigMap = actual.ArgoCDSSHKnownHostsConfigMap
	}
	if in.ArgoCDTLSCertsConfigMap == nil {
		in.ArgoCDTLSCertsConfigMap = actual.ArgoCDTLSCertsConfigMap
	}
	if in.ConfigManagementPlugins == nil {
		in.ConfigManagementPlugins = actual.ConfigManagementPlugins
	}

	return !cmp.Equal(before, in)
}

// isUpToDate compares managed spec against observed spec with
// structural equality; reduces to a standard cmp.Equal call now that
// both sides come from the codegen converters with consistent
// zero-value semantics.
func isUpToDate(desired, actual v1alpha2.InstanceParameters) bool {
	return cmp.Equal(desired, actual)
}

// buildApplyRequest materialises an ApplyInstanceRequest targeting the
// Akuity API. Non-ConfigMap payloads flow through the generated
// converters; ConfigMaps and plugins are marshalled directly via the
// existing protobuf helper.
func buildApplyRequest(_ context.Context, ac akuity.Client, mg *v1alpha2.Instance) (*argocdv1.ApplyInstanceRequest, error) {
	p := &mg.Spec.ForProvider
	if p.ArgoCD == nil {
		return nil, fmt.Errorf("managed Instance %q is missing spec.forProvider.argocd", mg.Name)
	}

	if ccd := p.ArgoCD.Spec.InstanceSpec.ClusterCustomizationDefaults; ccd != nil {
		if err := glue.ValidateKustomizationYAML(ccd.Kustomization); err != nil {
			return nil, fmt.Errorf("spec.forProvider.argocd.spec.instanceSpec.clusterCustomizationDefaults.kustomization: %w", err)
		}
	}

	argocdPB, err := argoCDToPB(p.Name, p.ArgoCD)
	if err != nil {
		return nil, err
	}

	cms := []struct {
		key  string
		data map[string]string
		dst  **structpb.Struct
	}{
		{argoCDCMKey, p.ArgoCDConfigMap, new(*structpb.Struct)},
		{argoCDRBACCMKey, p.ArgoCDRBACConfigMap, new(*structpb.Struct)},
		{argoCDNotificationsCMKey, p.ArgoCDNotificationsConfigMap, new(*structpb.Struct)},
		{argoCDImageUpdaterCMKey, p.ArgoCDImageUpdaterConfigMap, new(*structpb.Struct)},
		{argoCDImageUpdaterSSHCMKey, p.ArgoCDImageUpdaterSSHConfigMap, new(*structpb.Struct)},
		{argoCDSSHKnownHostsCMKey, p.ArgoCDSSHKnownHostsConfigMap, new(*structpb.Struct)},
		{argoCDTLSCertsCMKey, p.ArgoCDTLSCertsConfigMap, new(*structpb.Struct)},
	}
	for i := range cms {
		pb, err := configMapToPB(cms[i].key, cms[i].data)
		if err != nil {
			return nil, err
		}
		*cms[i].dst = pb
	}

	pluginsPB, err := pluginsToPB(p.ConfigManagementPlugins)
	if err != nil {
		return nil, err
	}

	_ = ac // the Akuity client injects OrganizationId + auth headers at call time; kept in the signature for symmetry with legacy builders.

	return &argocdv1.ApplyInstanceRequest{
		OrganizationId:            "", // Filled by the Akuity client wrapper via ApplyInstance(OrgID auto-injected).
		IdType:                    idv1.Type_NAME,
		Id:                        p.Name,
		Argocd:                    argocdPB,
		ArgocdConfigmap:           *cms[0].dst,
		ArgocdRbacConfigmap:       *cms[1].dst,
		NotificationsConfigmap:    *cms[2].dst,
		ImageUpdaterConfigmap:     *cms[3].dst,
		ImageUpdaterSshConfigmap:  *cms[4].dst,
		ArgocdKnownHostsConfigmap: *cms[5].dst,
		ArgocdTlsCertsConfigmap:   *cms[6].dst,
		ConfigManagementPlugins:   pluginsPB,
	}, nil
}

func argoCDToPB(name string, in *v1alpha2.ArgoCD) (*structpb.Struct, error) {
	argocd := akuitytypes.ArgoCD{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ArgoCD",
			APIVersion: "argocd.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: akuitytypes.ArgoCDSpec{
			Description: in.Spec.Description,
			Version:     in.Spec.Version,
			Shard:       in.Spec.Shard,
		},
	}
	if is := convert.InstanceSpecSpecToAPI(&in.Spec.InstanceSpec); is != nil {
		argocd.Spec.InstanceSpec = *is
	}
	pb, err := protobuf.MarshalObjectToProtobufStruct(argocd)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd spec to protobuf: %w", err)
	}
	return pb, nil
}

func configMapToPB(name string, data map[string]string) (*structpb.Struct, error) {
	if len(data) == 0 {
		return nil, nil
	}
	cm := corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       data,
	}
	pb, err := protobuf.MarshalObjectToProtobufStruct(cm)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s configmap to protobuf: %w", name, err)
	}
	return pb, nil
}

func configMapFromPB(name string, pb *structpb.Struct) (map[string]string, error) {
	if pb == nil {
		return nil, nil
	}
	cm := corev1.ConfigMap{}
	if err := protobuf.RemarshalObject(pb.AsMap(), &cm); err != nil {
		return nil, fmt.Errorf("could not unmarshal %s configmap from protobuf: %w", name, err)
	}
	if len(cm.Data) == 0 {
		return nil, nil
	}
	return cm.Data, nil
}

// pluginsToPB serialises the v1alpha2 plugin map into the
// []*structpb.Struct shape the Akuity API expects. v1alpha2 and
// upstream argocd_v1alpha1 share field shapes except for
// ParameterAnnouncement.String (v2) vs .String_ (wire), which we
// translate here.
func pluginsToPB(in map[string]v1alpha2.ConfigManagementPlugin) ([]*structpb.Struct, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]*structpb.Struct, 0, len(in))
	for name, p := range in {
		wire := argocdtypes.ConfigManagementPlugin{
			TypeMeta: metav1.TypeMeta{Kind: "ConfigManagementPlugin", APIVersion: "argocd.akuity.io/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					argocdtypes.AnnotationCMPEnabled: strconv.FormatBool(p.Enabled),
					argocdtypes.AnnotationCMPImage:   p.Image,
				},
			},
			Spec: argocdtypes.PluginSpec{
				Version:          p.Spec.Version,
				Init:             commandToWire(p.Spec.Init),
				Generate:         commandToWire(p.Spec.Generate),
				Discover:         discoverToWire(p.Spec.Discover),
				Parameters:       parametersToWire(p.Spec.Parameters),
				PreserveFileMode: p.Spec.PreserveFileMode,
			},
		}
		pb, err := protobuf.MarshalObjectToProtobufStruct(wire)
		if err != nil {
			return nil, fmt.Errorf("could not marshal plugin %q to protobuf: %w", name, err)
		}
		out = append(out, pb)
	}
	return out, nil
}

func pluginsFromPB(in []*structpb.Struct) (map[string]v1alpha2.ConfigManagementPlugin, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[string]v1alpha2.ConfigManagementPlugin, len(in))
	for _, pb := range in {
		wire := argocdtypes.ConfigManagementPlugin{}
		if err := protobuf.RemarshalObject(pb.AsMap(), &wire); err != nil {
			return nil, fmt.Errorf("could not unmarshal plugin from protobuf: %w", err)
		}
		out[wire.Name] = v1alpha2.ConfigManagementPlugin{
			Enabled: wire.Annotations[argocdtypes.AnnotationCMPEnabled] == "true",
			Image:   wire.Annotations[argocdtypes.AnnotationCMPImage],
			Spec: v1alpha2.PluginSpec{
				Version:          wire.Spec.Version,
				Init:             commandFromWire(wire.Spec.Init),
				Generate:         commandFromWire(wire.Spec.Generate),
				Discover:         discoverFromWire(wire.Spec.Discover),
				Parameters:       parametersFromWire(wire.Spec.Parameters),
				PreserveFileMode: wire.Spec.PreserveFileMode,
			},
		}
	}
	return out, nil
}

func commandToWire(c *v1alpha2.Command) *argocdtypes.Command {
	if c == nil {
		return nil
	}
	return &argocdtypes.Command{Command: c.Command, Args: c.Args}
}

func commandFromWire(c *argocdtypes.Command) *v1alpha2.Command {
	if c == nil {
		return nil
	}
	return &v1alpha2.Command{Command: c.Command, Args: c.Args}
}

func discoverToWire(d *v1alpha2.Discover) *argocdtypes.Discover {
	if d == nil {
		return nil
	}
	out := &argocdtypes.Discover{FileName: d.FileName}
	if d.Find != nil {
		out.Find = &argocdtypes.Find{Command: d.Find.Command, Args: d.Find.Args, Glob: d.Find.Glob}
	}
	return out
}

func discoverFromWire(d *argocdtypes.Discover) *v1alpha2.Discover {
	if d == nil {
		return nil
	}
	out := &v1alpha2.Discover{FileName: d.FileName}
	if d.Find != nil {
		out.Find = &v1alpha2.Find{Command: d.Find.Command, Args: d.Find.Args, Glob: d.Find.Glob}
	}
	return out
}

func parametersToWire(p *v1alpha2.Parameters) *argocdtypes.Parameters {
	if p == nil {
		return nil
	}
	out := &argocdtypes.Parameters{}
	if p.Dynamic != nil {
		out.Dynamic = &argocdtypes.Dynamic{Command: p.Dynamic.Command, Args: p.Dynamic.Args}
	}
	if p.Static != nil {
		out.Static = make([]*argocdtypes.ParameterAnnouncement, 0, len(p.Static))
		for _, s := range p.Static {
			if s == nil {
				continue
			}
			out.Static = append(out.Static, &argocdtypes.ParameterAnnouncement{
				Name: s.Name, Title: s.Title, Tooltip: s.Tooltip, Required: s.Required,
				ItemType: s.ItemType, CollectionType: s.CollectionType,
				String_: s.String, Array: s.Array, Map: s.Map,
			})
		}
	}
	return out
}

func parametersFromWire(p *argocdtypes.Parameters) *v1alpha2.Parameters {
	if p == nil {
		return nil
	}
	out := &v1alpha2.Parameters{}
	if p.Dynamic != nil {
		out.Dynamic = &v1alpha2.Dynamic{Command: p.Dynamic.Command, Args: p.Dynamic.Args}
	}
	if p.Static != nil {
		out.Static = make([]*v1alpha2.ParameterAnnouncement, 0, len(p.Static))
		for _, s := range p.Static {
			if s == nil {
				continue
			}
			out.Static = append(out.Static, &v1alpha2.ParameterAnnouncement{
				Name: s.Name, Title: s.Title, Tooltip: s.Tooltip, Required: s.Required,
				ItemType: s.ItemType, CollectionType: s.CollectionType,
				String: s.String_, Array: s.Array, Map: s.Map,
			})
		}
	}
	return out
}

// pbInstanceSpecToSpec bridges the argocdv1 protobuf InstanceSpec
// into the hand-authored akuitytypes.InstanceSpec that the
// codegen operates on, then lets the generated converter project it
// to v1alpha2. Going through marshal.ProtoToMap + RemarshalTo keeps
// field-name semantics aligned (protojson camelCase ↔ JSON tags).
func pbInstanceSpecToSpec(ai *argocdv1.Instance) (*v1alpha2.ArgoCDInstanceSpec, error) {
	pb := ai.GetSpec()
	if pb == nil {
		return nil, nil
	}
	m, err := marshal.ProtoToMap(pb)
	if err != nil {
		return nil, fmt.Errorf("instance spec protojson decode: %w", err)
	}
	wire := &akuitytypes.InstanceSpec{}
	if err := marshal.RemarshalTo(m, wire); err != nil {
		return nil, fmt.Errorf("instance spec remarshal: %w", err)
	}
	return convert.InstanceSpecAPIToSpec(wire), nil
}
