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
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base/children"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
	"github.com/akuityio/provider-crossplane-akuity/internal/secrets"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	argocdtypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/argocd/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/types/observation"
)

// argocdRepoSecretTypeLabel + the two const values below are the
// labels Argo CD uses to distinguish repository credentials from
// repo-credential templates. Matches terraform buildSecrets call sites
// in resource_akp_instance.go.
const (
	argocdRepoSecretTypeLabel        = "argocd.argoproj.io/secret-type"
	argocdRepoSecretTypeRepository   = "repository"
	argocdRepoSecretTypeRepoTemplate = "repo-creds"
)

// resolvedInstanceSecret holds one resolved singleton Secret reference
// (ref name + data) used by the four scalar refs on InstanceParameters.
type resolvedInstanceSecret struct {
	Name string
	Data map[string]string
}

// resolvedInstanceSecrets is the full resolution of every Secret
// referenced by InstanceParameters: four singletons (argocd / argocd-
// notifications / argocd-image-updater / applicationset) plus two
// Named lists (repoCredentialSecretRefs, repoTemplateCredentialSecretRefs).
// Controllers pass this alongside the bare spec into
// BuildApplyInstanceRequest so the Apply payload carries the actual
// Secret contents, not just references.
type resolvedInstanceSecrets struct {
	Argocd            resolvedInstanceSecret
	Notifications     resolvedInstanceSecret
	ImageUpdater      resolvedInstanceSecret
	ApplicationSet    resolvedInstanceSecret
	RepoCreds         map[string]map[string]string
	RepoTemplateCreds map[string]map[string]string
}

// Hash combines the digests of every resolved Secret, including the
// referenced Secret names, so a rename-with-identical-content rotates
// the digest as well as a straight content rotation. Empty when no
// refs were set on the spec — keeps Observe short-circuit cheap.
func (r resolvedInstanceSecrets) Hash() string {
	h := map[string]string{
		"argocd":            hashOneInstanceSecret(r.Argocd),
		"notifications":     hashOneInstanceSecret(r.Notifications),
		"imageUpdater":      hashOneInstanceSecret(r.ImageUpdater),
		"applicationSet":    hashOneInstanceSecret(r.ApplicationSet),
		"repoCreds":         secrets.HashNamed(r.RepoCreds),
		"repoTemplateCreds": secrets.HashNamed(r.RepoTemplateCreds),
	}
	for _, v := range h {
		if v != "" {
			return secrets.Hash(h)
		}
	}
	return ""
}

func hashOneInstanceSecret(s resolvedInstanceSecret) string {
	if s.Name == "" && len(s.Data) == 0 {
		return ""
	}
	return secrets.Hash(map[string]string{
		"__ref__":  s.Name,
		"__data__": secrets.Hash(s.Data),
	})
}

// instanceHasAnySecretRef reports whether the Instance spec declares any
// SecretRef at all. Lets resolveInstanceSecrets skip the PC lookup
// entirely when the user has not wired any credentials (the common
// minimal-spec path in 1C.B0).
func instanceHasAnySecretRef(mg *v1alpha1.Instance) bool {
	fp := mg.Spec.ForProvider
	return fp.ArgoCDSecretRef != nil ||
		fp.ArgoCDNotificationsSecretRef != nil ||
		fp.ArgoCDImageUpdaterSecretRef != nil ||
		fp.ApplicationSetSecretRef != nil ||
		len(fp.RepoCredentialSecretRefs) > 0 ||
		len(fp.RepoTemplateCredentialSecretRefs) > 0
}

// instanceSecretNamespace returns the namespace used to resolve the
// Instance's SecretRefs. Instance is cluster-scoped so mg.GetNamespace()
// is empty; fall back to the ProviderConfig's credentialsSecretRef
// namespace (the operator already nominated one to host credentials).
// For namespaced MRs the MR namespace wins. Returns an error when the
// MR carries no ProviderConfig reference — a configuration error the
// caller treats as terminal per §2.11 invariant 7.
func instanceSecretNamespace(ctx context.Context, kube client.Client, mg *v1alpha1.Instance) (string, error) {
	if ns := mg.GetNamespace(); ns != "" {
		return ns, nil
	}
	ref := mg.GetProviderConfigReference()
	if ref == nil {
		return "", fmt.Errorf("cluster-scoped Instance %q has no providerConfigRef; cannot resolve SecretRef namespace", mg.GetName())
	}
	pc := &apisv1alpha1.ProviderConfig{}
	if err := kube.Get(ctx, k8stypes.NamespacedName{Name: ref.Name}, pc); err != nil {
		return "", fmt.Errorf("cannot resolve SecretRef namespace: get ProviderConfig %q: %w", ref.Name, err)
	}
	if ns := pc.Spec.CredentialsSecretRef.Namespace; ns != "" {
		return ns, nil
	}
	return "", fmt.Errorf("ProviderConfig %q has no credentialsSecretRef.namespace to use as SecretRef lookup namespace", ref.Name)
}

// resolveInstanceSecrets loads every Secret referenced by the Instance
// spec from the namespace the MR lives in. Missing Secrets surface as
// a wrapped secrets.ErrMissingSecret so the controller can treat them
// as terminal configuration errors per §2.11 invariant 7.
func resolveInstanceSecrets(ctx context.Context, kube client.Client, mg *v1alpha1.Instance) (resolvedInstanceSecrets, error) {
	out := resolvedInstanceSecrets{}
	// Instance is cluster-scoped; LocalSecretReference carries no
	// namespace. Anchor the lookup on the MR namespace when present,
	// otherwise fall back to the ProviderConfig credentials namespace.
	if !instanceHasAnySecretRef(mg) {
		return out, nil
	}
	ns, err := instanceSecretNamespace(ctx, kube, mg)
	if err != nil {
		return out, err
	}
	fp := mg.Spec.ForProvider

	singletons := []struct {
		ref   *xpv1.LocalSecretReference
		out   *resolvedInstanceSecret
		label string
	}{
		{fp.ArgoCDSecretRef, &out.Argocd, "argocdSecretRef"},
		{fp.ArgoCDNotificationsSecretRef, &out.Notifications, "argocdNotificationsSecretRef"},
		{fp.ArgoCDImageUpdaterSecretRef, &out.ImageUpdater, "argocdImageUpdaterSecretRef"},
		{fp.ApplicationSetSecretRef, &out.ApplicationSet, "applicationSetSecretRef"},
	}
	for _, s := range singletons {
		data, err := secrets.ResolveAllKeys(ctx, kube, ns, s.ref)
		if err != nil {
			return out, fmt.Errorf("%s: %w", s.label, err)
		}
		if s.ref != nil {
			s.out.Name = s.ref.Name
		}
		s.out.Data = data
	}

	if refs := fp.RepoCredentialSecretRefs; len(refs) > 0 {
		d, err := secrets.ResolveNamed(ctx, kube, ns, refs)
		if err != nil {
			return out, fmt.Errorf("repoCredentialSecretRefs: %w", err)
		}
		out.RepoCreds = d
	}
	if refs := fp.RepoTemplateCredentialSecretRefs; len(refs) > 0 {
		d, err := secrets.ResolveNamed(ctx, kube, ns, refs)
		if err != nil {
			return out, fmt.Errorf("repoTemplateCredentialSecretRefs: %w", err)
		}
		out.RepoTemplateCreds = d
	}
	return out, nil
}

// instanceSecretToPB marshals a resolved singleton Secret into the
// Kubernetes Secret structpb.Struct that the gateway expects. Returns
// nil with no error when data is empty so callers can compose freely.
func instanceSecretToPB(name string, data map[string]string, labels map[string]string) (*structpb.Struct, error) {
	if len(data) == 0 {
		return nil, nil
	}
	byt := make(map[string][]byte, len(data))
	for k, v := range data {
		byt[k] = []byte(v)
	}
	sec := corev1.Secret{
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Data: byt,
	}
	pb, err := marshal.APIModelToPBStruct(sec)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s secret to protobuf: %w", name, err)
	}
	return pb, nil
}

// namedInstanceSecretsToPB serialises each entry in a Named-ref map
// into a labelled Kubernetes Secret. Entries are emitted in sorted
// name order so the Apply payload is byte-identical across reconciles
// for the same input. Empty entries are skipped; empty input returns
// nil.
func namedInstanceSecretsToPB(named map[string]map[string]string, label string) ([]*structpb.Struct, error) {
	if len(named) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(named))
	for n := range named {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]*structpb.Struct, 0, len(named))
	for _, n := range names {
		pb, err := instanceSecretToPB(n, named[n], map[string]string{argocdRepoSecretTypeLabel: label})
		if err != nil {
			return nil, fmt.Errorf("%s %q: %w", label, n, err)
		}
		if pb == nil {
			continue
		}
		out = append(out, pb)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// BuildApplyInstanceRequest assembles the ApplyInstanceRequest proto
// from the managed Instance's forProvider spec plus a resolvedInstance-
// Secrets bundle produced by resolveInstanceSecrets. OrganizationId is
// left empty and filled by the client's ApplyInstance method so this
// function stays pure and testable. Exported so tests can round-trip
// through the same builder the controller uses; tests that do not
// exercise Secret wiring pass a zero-value resolvedInstanceSecrets{}.
//
//nolint:gocyclo // Linear pipeline over 7 ConfigMaps + 4 singleton Secrets + 2 Named-ref lists + ArgoCD spec + CMPs; splitting would yield 14 trivial wrappers without clarity gain.
func BuildApplyInstanceRequest(instance v1alpha1.Instance, sec resolvedInstanceSecrets) (*argocdv1.ApplyInstanceRequest, error) {
	argocdPB, err := specToArgoCDPB(instance.Spec.ForProvider.Name, instance.Spec.ForProvider.ArgoCD)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd spec to protobuf: %w", err)
	}

	argocdConfigMapPB, err := specToConfigMapPB(observation.ArgocdCMKey, instance.Spec.ForProvider.ArgoCDConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd configmap to protobuf: %w", err)
	}

	argocdRbacConfigMapPB, err := specToConfigMapPB(observation.ArgocdRBACCMKey, instance.Spec.ForProvider.ArgoCDRBACConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd rbac configmap to protobuf: %w", err)
	}

	argocdNotificationsConfigMapPB, err := specToConfigMapPB(observation.ArgocdNotificationsCMKey, instance.Spec.ForProvider.ArgoCDNotificationsConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd notifications configmap to protobuf: %w", err)
	}

	argocdImageUpdaterConfigMapPB, err := specToConfigMapPB(observation.ArgocdImageUpdaterCMKey, instance.Spec.ForProvider.ArgoCDImageUpdaterConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd image updater configmap to protobuf: %w", err)
	}

	argocdImageUpdaterSshConfigMapPB, err := specToConfigMapPB(observation.ArgocdImageUpdaterSSHCMKey, instance.Spec.ForProvider.ArgoCDImageUpdaterSSHConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd image updater ssh configmap to protobuf: %w", err)
	}

	argocdKnownHostsConfigMapPB, err := specToConfigMapPB(observation.ArgocdSSHKnownHostsCMKey, instance.Spec.ForProvider.ArgoCDSSHKnownHostsConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd known hosts configmap to protobuf: %w", err)
	}

	argocdTlsCertsConfigMapPB, err := specToConfigMapPB(observation.ArgocdTLSCertsCMKey, instance.Spec.ForProvider.ArgoCDTLSCertsConfigMap)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd tls certs configmap to protobuf: %w", err)
	}

	configManagementPluginsPB, err := specToConfigManagementPluginsPB(instance.Spec.ForProvider.ConfigManagementPlugins)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd config management plugins to protobuf: %w", err)
	}

	argocdSecretPB, err := instanceSecretToPB(observation.ArgocdSecretKey, sec.Argocd.Data, nil)
	if err != nil {
		return nil, err
	}
	notificationsSecretPB, err := instanceSecretToPB(observation.ArgocdNotificationsSecretKey, sec.Notifications.Data, nil)
	if err != nil {
		return nil, err
	}
	imageUpdaterSecretPB, err := instanceSecretToPB(observation.ArgocdImageUpdaterSecretKey, sec.ImageUpdater.Data, nil)
	if err != nil {
		return nil, err
	}
	applicationSetSecretPB, err := instanceSecretToPB(observation.ArgocdApplicationSetKey, sec.ApplicationSet.Data, nil)
	if err != nil {
		return nil, err
	}
	repoCredsPB, err := namedInstanceSecretsToPB(sec.RepoCreds, argocdRepoSecretTypeRepository)
	if err != nil {
		return nil, err
	}
	repoTemplateCredsPB, err := namedInstanceSecretsToPB(sec.RepoTemplateCreds, argocdRepoSecretTypeRepoTemplate)
	if err != nil {
		return nil, err
	}

	argocdChildren, err := splitArgocdResources(instance.Spec.ForProvider.Resources)
	if err != nil {
		return nil, err
	}

	return &argocdv1.ApplyInstanceRequest{
		IdType:                        idv1.Type_NAME,
		Id:                            instance.Spec.ForProvider.Name,
		Argocd:                        argocdPB,
		ArgocdConfigmap:               argocdConfigMapPB,
		ArgocdRbacConfigmap:           argocdRbacConfigMapPB,
		NotificationsConfigmap:        argocdNotificationsConfigMapPB,
		ImageUpdaterConfigmap:         argocdImageUpdaterConfigMapPB,
		ImageUpdaterSshConfigmap:      argocdImageUpdaterSshConfigMapPB,
		ArgocdKnownHostsConfigmap:     argocdKnownHostsConfigMapPB,
		ArgocdTlsCertsConfigmap:       argocdTlsCertsConfigMapPB,
		ConfigManagementPlugins:       configManagementPluginsPB,
		ArgocdSecret:                  argocdSecretPB,
		NotificationsSecret:           notificationsSecretPB,
		ImageUpdaterSecret:            imageUpdaterSecretPB,
		ApplicationSetSecret:          applicationSetSecretPB,
		RepoCredentialSecrets:         repoCredsPB,
		RepoTemplateCredentialSecrets: repoTemplateCredsPB,
		Applications:                  argocdChildren.Applications,
		ApplicationSets:               argocdChildren.ApplicationSets,
		AppProjects:                   argocdChildren.AppProjects,
	}, nil
}

func specToArgoCDPB(name string, instance *crossplanetypes.ArgoCD) (*structpb.Struct, error) {
	instanceSpec, err := SpecToInstanceSpec(instance.Spec.InstanceSpec)
	if err != nil {
		return nil, err
	}

	argocd := akuitytypes.ArgoCD{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ArgoCD",
			APIVersion: "argocd.akuity.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: akuitytypes.ArgoCDSpec{
			Description:  instance.Spec.Description,
			Version:      instance.Spec.Version,
			InstanceSpec: instanceSpec,
		},
	}

	argocdPB, err := marshal.APIModelToPBStruct(argocd)
	if err != nil {
		return nil, fmt.Errorf("could not marshal instance argocd spec to protobuf: %w", err)
	}

	return argocdPB, nil
}

func SpecToInstanceSpec(instanceSpec crossplanetypes.InstanceSpec) (akuitytypes.InstanceSpec, error) {
	clusterCustomization, err := specToClusterCustomization(instanceSpec.ClusterCustomizationDefaults)
	if err != nil {
		return akuitytypes.InstanceSpec{}, fmt.Errorf("could not build instance argocd instance spec: %w", err)
	}

	appReconciliationsRateLimiting, err := specToAppReconciliationsRateLimiting(instanceSpec.AppReconciliationsRateLimiting)
	if err != nil {
		return akuitytypes.InstanceSpec{}, fmt.Errorf("could not build instance app reconciliations rate limiting config: %w", err)
	}

	return akuitytypes.InstanceSpec{
		IpAllowList:                     specToIPAllowList(instanceSpec.IpAllowList),
		Subdomain:                       instanceSpec.Subdomain,
		DeclarativeManagementEnabled:    instanceSpec.DeclarativeManagementEnabled,
		Extensions:                      specToExtensionInstallEntries(instanceSpec.Extensions),
		ClusterCustomizationDefaults:    clusterCustomization,
		ImageUpdaterEnabled:             instanceSpec.ImageUpdaterEnabled,
		BackendIpAllowListEnabled:       instanceSpec.BackendIpAllowListEnabled,
		RepoServerDelegate:              specToRepoServerDelegate(instanceSpec.RepoServerDelegate),
		AuditExtensionEnabled:           instanceSpec.AuditExtensionEnabled,
		SyncHistoryExtensionEnabled:     instanceSpec.SyncHistoryExtensionEnabled,
		CrossplaneExtension:             specToCrossplaneExtension(instanceSpec.CrossplaneExtension),
		ImageUpdaterDelegate:            specToImageUpdaterDelegate(instanceSpec.ImageUpdaterDelegate),
		AppSetDelegate:                  specToAppSetDelegate(instanceSpec.AppSetDelegate),
		AssistantExtensionEnabled:       instanceSpec.AssistantExtensionEnabled,
		AppsetPolicy:                    specToAppsetPolicy(instanceSpec.AppsetPolicy),
		HostAliases:                     specToHostAliases(instanceSpec.HostAliases),
		AgentPermissionsRules:           specToAgentPermissionsRules(instanceSpec.AgentPermissionsRules),
		Fqdn:                            instanceSpec.Fqdn,
		MultiClusterK8SDashboardEnabled: instanceSpec.MultiClusterK8SDashboardEnabled,
		AkuityIntelligenceExtension:     specToAkuityIntelligenceExtension(instanceSpec.AkuityIntelligenceExtension),
		ImageUpdaterVersion:             instanceSpec.ImageUpdaterVersion,
		CustomDeprecatedApis:            specToCustomDeprecatedApis(instanceSpec.CustomDeprecatedApis),
		KubeVisionConfig:                specToKubeVisionConfig(instanceSpec.KubeVisionConfig),
		AppInAnyNamespaceConfig:         specToAppInAnyNamespaceConfig(instanceSpec.AppInAnyNamespaceConfig),
		Basepath:                        instanceSpec.Basepath,
		AppsetProgressiveSyncsEnabled:   instanceSpec.AppsetProgressiveSyncsEnabled,
		AppsetPlugins:                   specToAppsetPlugins(instanceSpec.AppsetPlugins),
		ApplicationSetExtension:         specToApplicationSetExtension(instanceSpec.ApplicationSetExtension),
		AppReconciliationsRateLimiting:  appReconciliationsRateLimiting,
	}, nil
}

func specToIPAllowList(ipAllowList []*crossplanetypes.IPAllowListEntry) []*akuitytypes.IPAllowListEntry {
	out := make([]*akuitytypes.IPAllowListEntry, 0, len(ipAllowList))
	for _, i := range ipAllowList {
		out = append(out, &akuitytypes.IPAllowListEntry{
			Ip:          i.Ip,
			Description: i.Description,
		})
	}
	return out
}

func specToExtensionInstallEntries(list []*crossplanetypes.ArgoCDExtensionInstallEntry) []*akuitytypes.ArgoCDExtensionInstallEntry {
	out := make([]*akuitytypes.ArgoCDExtensionInstallEntry, 0, len(list))
	for _, i := range list {
		out = append(out, &akuitytypes.ArgoCDExtensionInstallEntry{
			Id:      i.Id,
			Version: i.Version,
		})
	}
	return out
}

func specToClusterCustomization(in *crossplanetypes.ClusterCustomization) (*akuitytypes.ClusterCustomization, error) {
	if in == nil {
		return nil, nil
	}
	kustomization := runtime.RawExtension{}
	if err := yaml.Unmarshal([]byte(in.Kustomization), &kustomization); err != nil {
		return nil, fmt.Errorf("could not unmarshal cluster Kustomization from YAML to runtime raw extension: %w", err)
	}
	return &akuitytypes.ClusterCustomization{
		AutoUpgradeDisabled: in.AutoUpgradeDisabled,
		Kustomization:       kustomization,
		AppReplication:      in.AppReplication,
		RedisTunneling:      in.RedisTunneling,
	}, nil
}

func specToRepoServerDelegate(in *crossplanetypes.RepoServerDelegate) *akuitytypes.RepoServerDelegate {
	if in == nil {
		return nil
	}
	return &akuitytypes.RepoServerDelegate{
		ControlPlane: in.ControlPlane,
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: in.ManagedCluster.ClusterName,
		},
	}
}

func specToCrossplaneExtension(in *crossplanetypes.CrossplaneExtension) *akuitytypes.CrossplaneExtension {
	if in == nil {
		return nil
	}
	resources := make([]*akuitytypes.CrossplaneExtensionResource, 0, len(in.Resources))
	for _, r := range in.Resources {
		resources = append(resources, &akuitytypes.CrossplaneExtensionResource{Group: r.Group})
	}
	return &akuitytypes.CrossplaneExtension{Resources: resources}
}

func specToImageUpdaterDelegate(in *crossplanetypes.ImageUpdaterDelegate) *akuitytypes.ImageUpdaterDelegate {
	if in == nil {
		return nil
	}
	return &akuitytypes.ImageUpdaterDelegate{
		ControlPlane: in.ControlPlane,
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: in.ManagedCluster.ClusterName,
		},
	}
}

func specToAppSetDelegate(in *crossplanetypes.AppSetDelegate) *akuitytypes.AppSetDelegate {
	if in == nil {
		return nil
	}
	return &akuitytypes.AppSetDelegate{
		ManagedCluster: &akuitytypes.ManagedCluster{
			ClusterName: in.ManagedCluster.ClusterName,
		},
	}
}

func specToAppsetPolicy(in *crossplanetypes.AppsetPolicy) *akuitytypes.AppsetPolicy {
	if in == nil {
		return nil
	}
	return &akuitytypes.AppsetPolicy{
		Policy:         in.Policy,
		OverridePolicy: in.OverridePolicy,
	}
}

func specToHostAliases(list []*crossplanetypes.HostAliases) []*akuitytypes.HostAliases {
	out := make([]*akuitytypes.HostAliases, 0, len(list))
	for _, h := range list {
		out = append(out, &akuitytypes.HostAliases{
			Ip:        h.Ip,
			Hostnames: h.Hostnames,
		})
	}
	return out
}

func specToAgentPermissionsRules(rules []*crossplanetypes.AgentPermissionsRule) []*akuitytypes.AgentPermissionsRule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]*akuitytypes.AgentPermissionsRule, 0, len(rules))
	for _, r := range rules {
		copied := r.DeepCopy()
		out = append(out, &akuitytypes.AgentPermissionsRule{
			ApiGroups: copied.ApiGroups,
			Resources: copied.Resources,
			Verbs:     copied.Verbs,
		})
	}
	return out
}

func specToConfigMapPB(name string, data map[string]string) (*structpb.Struct, error) {
	if len(data) == 0 {
		return nil, nil
	}
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       data,
	}
	pb, err := marshal.APIModelToPBStruct(cm)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s configmap to protobuf struct: %w", name, err)
	}
	return pb, nil
}

func specToConfigManagementPluginsPB(plugins map[string]crossplanetypes.ConfigManagementPlugin) ([]*structpb.Struct, error) {
	out := make([]*structpb.Struct, 0)

	for name, plugin := range plugins {
		static := make([]*argocdtypes.ParameterAnnouncement, 0)
		for _, pm := range plugin.Spec.Parameters.Static {
			static = append(static, &argocdtypes.ParameterAnnouncement{
				Name:           pm.Name,
				Title:          pm.Title,
				Tooltip:        pm.Tooltip,
				Required:       ptr.To(pm.Required),
				ItemType:       pm.ItemType,
				CollectionType: pm.CollectionType,
				String_:        pm.String_,
				Array:          pm.Array,
				Map:            pm.Map,
			})
		}

		cmp := argocdtypes.ConfigManagementPlugin{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigManagementPlugin",
				APIVersion: "argocd.akuity.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					argocdtypes.AnnotationCMPEnabled: strconv.FormatBool(plugin.Enabled),
					argocdtypes.AnnotationCMPImage:   plugin.Image,
				},
			},
			Spec: argocdtypes.PluginSpec{
				Version:  plugin.Spec.Version,
				Init:     (*argocdtypes.Command)(plugin.Spec.Init),
				Generate: (*argocdtypes.Command)(plugin.Spec.Generate),
				Discover: &argocdtypes.Discover{
					Find:     (*argocdtypes.Find)(plugin.Spec.Discover.Find),
					FileName: plugin.Spec.Discover.FileName,
				},
				Parameters: &argocdtypes.Parameters{
					Static:  static,
					Dynamic: (*argocdtypes.Dynamic)(plugin.Spec.Parameters.Dynamic),
				},
				PreserveFileMode: ptr.To(plugin.Spec.PreserveFileMode),
			},
		}

		pb, err := marshal.APIModelToPBStruct(cmp)
		if err != nil {
			return nil, fmt.Errorf("could not marshal %s config management plugin to protobuf struct: %w", name, err)
		}
		out = append(out, pb)
	}

	return out, nil
}

func specToAkuityIntelligenceExtension(in *crossplanetypes.AkuityIntelligenceExtension) *akuitytypes.AkuityIntelligenceExtension {
	if in == nil {
		return nil
	}
	return &akuitytypes.AkuityIntelligenceExtension{
		Enabled:                  in.Enabled,
		AllowedUsernames:         in.AllowedUsernames,
		AllowedGroups:            in.AllowedGroups,
		AiSupportEngineerEnabled: in.AiSupportEngineerEnabled,
		ModelVersion:             in.ModelVersion,
	}
}

func specToCustomDeprecatedApis(list []*crossplanetypes.CustomDeprecatedAPI) []*akuitytypes.CustomDeprecatedAPI {
	if len(list) == 0 {
		return nil
	}
	out := make([]*akuitytypes.CustomDeprecatedAPI, 0, len(list))
	for _, c := range list {
		out = append(out, &akuitytypes.CustomDeprecatedAPI{
			ApiVersion:                     c.ApiVersion,
			NewApiVersion:                  c.NewApiVersion,
			DeprecatedInKubernetesVersion:  c.DeprecatedInKubernetesVersion,
			UnavailableInKubernetesVersion: c.UnavailableInKubernetesVersion,
		})
	}
	return out
}

func specToKubeVisionConfig(in *crossplanetypes.KubeVisionConfig) *akuitytypes.KubeVisionConfig {
	if in == nil {
		return nil
	}
	return &akuitytypes.KubeVisionConfig{
		CveScanConfig: &akuitytypes.CveScanConfig{
			ScanEnabled:    in.CveScanConfig.ScanEnabled,
			RescanInterval: in.CveScanConfig.RescanInterval,
		},
	}
}

func specToAppInAnyNamespaceConfig(in *crossplanetypes.AppInAnyNamespaceConfig) *akuitytypes.AppInAnyNamespaceConfig {
	if in == nil {
		return nil
	}
	return &akuitytypes.AppInAnyNamespaceConfig{
		Enabled: in.Enabled,
	}
}

func specToAppsetPlugins(list []*crossplanetypes.AppsetPlugins) []*akuitytypes.AppsetPlugins {
	if len(list) == 0 {
		return nil
	}
	out := make([]*akuitytypes.AppsetPlugins, 0, len(list))
	for _, p := range list {
		out = append(out, &akuitytypes.AppsetPlugins{
			Name:           p.Name,
			Token:          p.Token,
			BaseUrl:        p.BaseUrl,
			RequestTimeout: p.RequestTimeout,
		})
	}
	return out
}

func specToApplicationSetExtension(in *crossplanetypes.ApplicationSetExtension) *akuitytypes.ApplicationSetExtension {
	if in == nil {
		return nil
	}
	return &akuitytypes.ApplicationSetExtension{
		Enabled: in.Enabled,
	}
}

func specToAppReconciliationsRateLimiting(in *crossplanetypes.AppReconciliationsRateLimiting) (*akuitytypes.AppReconciliationsRateLimiting, error) {
	if in == nil {
		return nil, nil
	}
	rl := &akuitytypes.AppReconciliationsRateLimiting{}

	if in.BucketRateLimiting != nil {
		bucket := in.BucketRateLimiting
		rl.BucketRateLimiting = &akuitytypes.BucketRateLimiting{
			Enabled:    bucket.Enabled,
			BucketSize: bucket.BucketSize,
			BucketQps:  bucket.BucketQps,
		}
	}

	if in.ItemRateLimiting != nil {
		item := in.ItemRateLimiting
		rl.ItemRateLimiting = &akuitytypes.ItemRateLimiting{
			Enabled:         item.Enabled,
			FailureCooldown: item.FailureCooldown,
			BaseDelay:       item.BaseDelay,
			MaxDelay:        item.MaxDelay,
		}

		if item.BackoffFactorString != "" {
			backoff, err := strconv.ParseFloat(item.BackoffFactorString, 32)
			if err != nil {
				return nil, fmt.Errorf("could not parse backoff factor %q as float: %w", item.BackoffFactorString, err)
			}
			rl.ItemRateLimiting.BackoffFactor = float32(backoff)
		}
	}

	return rl, nil
}

// Declarative Argo CD child-resource contract. Each entry in
// spec.forProvider.resources must carry one of these
// (apiVersion, kind) pairs; anything else is rejected at reconcile
// entry. Matches the kinds the Akuity gateway's ApplyInstance proto
// accepts on its Applications / ApplicationSets / AppProjects slices.
const (
	argocdAPIVersion           = "argoproj.io/v1alpha1"
	argocdKindApplication      = "Application"
	argocdKindApplicationSet   = "ApplicationSet"
	argocdKindAppProject       = "AppProject"
	errSecretInArgocdResources = "resources[%d]: v1/Secret entries are not accepted; use spec.forProvider.argocdSecretRef or spec.forProvider.repoCredentialSecretRefs"
)

// argocdChildren is the per-kind breakdown of the user's
// spec.forProvider.resources bundle, already marshalled into the
// structpb.Struct shape the ApplyInstance proto expects on its
// Applications / ApplicationSets / AppProjects slices. Mirrors the
// kargoChildren shape on the KargoInstance controller — the wire shape
// is the only thing that differs.
type argocdChildren struct {
	Applications    []*structpb.Struct
	ApplicationSets []*structpb.Struct
	AppProjects     []*structpb.Struct
}

// splitArgocdResources validates each spec.forProvider.resources
// entry and routes it into argocdChildren by (apiVersion, kind).
// Empty input yields a zero struct and no error so callers can
// compose without pre-checks. Mirrors splitKargoResources on the
// KargoInstance controller.
//
// Inline v1/Secret entries are rejected as a terminal error: storing
// plaintext credential data on an MR spec is the very thing the typed
// SecretRef fields exist to avoid. The terminal classification halts
// the reconcile loop on the bad input rather than retrying it on
// every poll while admission controllers complain.
func splitArgocdResources(in []runtime.RawExtension) (argocdChildren, error) {
	out := argocdChildren{}
	if len(in) == 0 {
		return out, nil
	}
	for i, raw := range in {
		if err := routeArgocdResource(&out, i, raw); err != nil {
			return out, err
		}
	}
	return out, nil
}

// routeArgocdResource decodes a single resources[i] entry, runs the
// allowlist + Secret-rejection checks, and appends the encoded
// structpb onto the matching kind slice. Split out from
// splitArgocdResources so the per-entry validation pipeline stays
// readable without inflating the parent's cyclomatic complexity.
func routeArgocdResource(out *argocdChildren, i int, raw runtime.RawExtension) error {
	if len(raw.Raw) == 0 {
		return fmt.Errorf("resources[%d]: empty payload", i)
	}
	obj := map[string]interface{}{}
	if err := json.Unmarshal(raw.Raw, &obj); err != nil {
		return fmt.Errorf("resources[%d]: invalid JSON: %w", i, err)
	}
	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)
	if apiVersion == "v1" && kind == "Secret" {
		return reason.AsTerminal(fmt.Errorf(errSecretInArgocdResources, i))
	}
	pb, err := structpb.NewStruct(obj)
	if err != nil {
		return fmt.Errorf("resources[%d]: structpb encode: %w", i, err)
	}
	if apiVersion != argocdAPIVersion {
		return fmt.Errorf("resources[%d]: unsupported %s/%s", i, apiVersion, kind)
	}
	switch kind {
	case argocdKindApplication:
		out.Applications = append(out.Applications, pb)
	case argocdKindApplicationSet:
		out.ApplicationSets = append(out.ApplicationSets, pb)
	case argocdKindAppProject:
		out.AppProjects = append(out.AppProjects, pb)
	default:
		return fmt.Errorf("resources[%d]: unsupported %s/%s", i, apiVersion, kind)
	}
	return nil
}

// argocdResourcesUpToDate reports whether every declarative Argo CD
// child listed in spec.forProvider.resources is present on the gateway
// with an equivalent payload. Mirrors kargoResourcesUpToDate on the
// KargoInstance controller: additive semantics, desired ⊆ observed.
// Removing an entry from spec does NOT trigger server-side deletion —
// out-of-band resources managed via the Akuity UI must not be wiped
// by a missing entry on the Crossplane side.
func argocdResourcesUpToDate(desired []runtime.RawExtension, exp *argocdv1.ExportInstanceResponse) (bool, children.DriftReport, error) {
	if len(desired) == 0 {
		return true, children.DriftReport{}, nil
	}
	desiredIdx, err := children.Index(desired)
	if err != nil {
		return false, children.DriftReport{}, fmt.Errorf("resources: %w", err)
	}
	observedAll := make(map[children.Identity]map[string]interface{})
	groups := [][]*structpb.Struct{
		exp.GetApplications(),
		exp.GetApplicationSets(),
		exp.GetAppProjects(),
	}
	for _, group := range groups {
		group := group
		idx, err := children.IndexStructs(group)
		if err != nil {
			// Defer the failure to the Apply path rather than failing
			// the reconcile loop on a transient decode issue.
			//nolint:nilerr // intentional swallow; see comment above
			return true, children.DriftReport{}, nil
		}
		for k, v := range idx {
			observedAll[k] = v
		}
	}
	report := children.Compare(desiredIdx, observedAll)
	return report.Empty(), report, nil
}
