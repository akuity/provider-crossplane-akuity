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
	"reflect"
	"sort"
	"strconv"

	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	idv1 "github.com/akuity/api-client-go/pkg/api/gen/types/id/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/children"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert/glue"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	"github.com/akuityio/provider-crossplane-akuity/internal/secrets"
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

// Well-known secret names used in the Akuity apply payload. Entry names
// match the ArgoCD conventions the gateway expects; the controller
// populates each from a user-supplied xpv1.LocalSecretReference at
// reconcile time, so plaintext never lives on the managed resource.
const (
	argoCDSecretKey              = "argocd-secret"
	argoCDNotificationsSecretKey = "argocd-notifications-secret"
	argoCDImageUpdaterSecretKey  = "argocd-image-updater-secret"
	applicationSetSecretKey      = "argocd-application-set-secret"
	secretTypeLabel              = "argocd.argoproj.io/secret-type"
	secretTypeRepository         = "repository"
	secretTypeRepoCreds          = "repo-creds"
)

// Declarative ArgoCD child-resource contract. Each entry in
// spec.forProvider.argocdResources must carry one of these apiVersion /
// kind pairs; anything else is rejected at reconcile entry.
const (
	argoprojAPIVersion     = "argoproj.io/v1alpha1"
	argoKindApplication    = "Application"
	argoKindApplicationSet = "ApplicationSet"
	argoKindAppProject     = "AppProject"
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

	obs.ApplicationsStatus = applicationsStatusFromPB(ai.GetInfo().GetApplicationsStatus())

	return obs
}

// applicationsStatusFromPB projects the Akuity InstanceInfo
// applications-status counters into the v1alpha2 AtProvider shape.
// Returns nil when the gateway reports no aggregate.
func applicationsStatusFromPB(in *argocdv1.ApplicationsStatus) *v1alpha2.ApplicationsStatus {
	if in == nil {
		return nil
	}
	out := &v1alpha2.ApplicationsStatus{
		ApplicationCount:    in.GetApplicationCount(),
		ResourcesCount:      in.GetResourcesCount(),
		AppOfAppCount:       in.GetAppOfAppCount(),
		SyncInProgressCount: in.GetSyncInProgressCount(),
		WarningCount:        in.GetWarningCount(),
		ErrorCount:          in.GetErrorCount(),
	}
	if h := in.GetHealth(); h != nil {
		out.Health = &v1alpha2.ApplicationsHealth{
			HealthyCount:     h.GetHealthyCount(),
			DegradedCount:    h.GetDegradedCount(),
			ProgressingCount: h.GetProgressingCount(),
			UnknownCount:     h.GetUnknownCount(),
			SuspendedCount:   h.GetSuspendedCount(),
			MissingCount:     h.GetMissingCount(),
		}
	}
	if s := in.GetSyncStatus(); s != nil {
		out.SyncStatus = &v1alpha2.ApplicationsSyncStatus{
			SyncedCount:    s.GetSyncedCount(),
			OutOfSyncCount: s.GetOutOfSyncCount(),
			UnknownCount:   s.GetUnknownCount(),
		}
	}
	return out
}

// carryOverSensitiveRefs copies the Instance's sensitive-secret
// references from the desired spec into the observed spec. The gateway
// masks secret contents on Get/Export and never returns ref fields at
// all, so without this carry-over every reconcile would see apparent
// drift and fire a redundant Apply. Rotation of the backing Secret is
// still detected via the secret-hash value on AtProvider.
func carryOverSensitiveRefs(desired, actual *v1alpha2.InstanceParameters) {
	actual.ArgoCDSecretRef = desired.ArgoCDSecretRef
	actual.ArgoCDNotificationsSecretRef = desired.ArgoCDNotificationsSecretRef
	actual.ArgoCDImageUpdaterSecretRef = desired.ArgoCDImageUpdaterSecretRef
	actual.ApplicationSetSecretRef = desired.ApplicationSetSecretRef
	actual.RepoCredentialSecretRefs = desired.RepoCredentialSecretRefs
	actual.RepoTemplateCredentialSecretRefs = desired.RepoTemplateCredentialSecretRefs
}

// lateInitialize fills in defaults from the actual (observed) instance
// into the managed spec for fields the user commonly omits. Returns
// true when any field on in was mutated so the caller can signal
// ResourceLateInitialized to the managed.Reconciler.
//
// ArgoCDInstanceSpec has ~13 pointer fields that the Akuity server
// populates with defaults on first Apply (e.g. DeclarativeManagement
// Enabled=true, ClusterCustomizationDefaults, KubeVisionConfig, ...).
// If those defaults are never copied back into the managed resource's
// spec, every subsequent reconcile sees nil-vs-populated drift and
// fires another Apply — the customer-reported infinite-reconcile
// pattern. Rather than listing each field explicitly (and forgetting
// the next one the server adds), walk the struct reflectively and
// adopt any nil→non-nil pointer and "" → non-empty string field.
func lateInitialize(in, actual *v1alpha2.InstanceParameters) bool {
	before := in.DeepCopy()

	if in.ArgoCD != nil && actual.ArgoCD != nil {
		lateInitializeStruct(
			reflect.ValueOf(&in.ArgoCD.Spec.InstanceSpec).Elem(),
			reflect.ValueOf(&actual.ArgoCD.Spec.InstanceSpec).Elem(),
		)
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

// lateInitializeStruct walks two matching structs and, for every field
// that is unset on `in` but populated on `actual`, copies actual into
// in. Handles pointer fields (nil → non-nil) and string fields
// ("" → non-empty). Leaves already-set fields untouched so that a
// user's explicit value wins over a server default.
//
//nolint:gocyclo // Reflect-based late-init mirrors the wire shape; one branch per supported reflect.Kind keeps the dispatch readable.
func lateInitializeStruct(in, actual reflect.Value) {
	if in.Kind() != reflect.Struct || actual.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < in.NumField(); i++ {
		inField := in.Field(i)
		acField := actual.Field(i)
		if !inField.CanSet() {
			continue
		}
		switch inField.Kind() { //nolint:exhaustive // only kinds late-initialised by upstream wire are handled; everything else is a primitive that the controller writes directly.
		case reflect.Pointer, reflect.Slice, reflect.Map:
			if inField.IsNil() && !acField.IsNil() {
				inField.Set(acField)
			}
		case reflect.String:
			if inField.Len() == 0 && acField.Len() > 0 {
				inField.Set(acField)
			}
		}
	}
}

// isUpToDate compares managed spec against observed spec with
// structural equality. `*bool` fields are compared by their effective
// boolean value (nil and &false are treated as equal) because the
// Akuity proto3 wire drops `false` as the primitive-bool zero value —
// observed state round-trips back as `nil` even when the user wrote
// `false` into their spec. Treating the two as equal avoids spec ==
// &false vs observed == nil being a permanent apparent drift that
// fires `ApplyInstance` every poll.
//
// ArgocdResources is intentionally excluded from this comparison: the
// opaque RawExtension payload cannot be round-tripped faithfully from
// the server's parsed Applications/ApplicationSets/AppProjects lists.
// Drift for that bundle is computed separately in Observe via the
// children.Compare helper against the ExportInstance response.
func isUpToDate(desired, actual v1alpha2.InstanceParameters) bool {
	return cmp.Equal(desired, actual,
		cmp.Comparer(boolPtrEqualEffective),
		cmpopts.IgnoreFields(v1alpha2.InstanceParameters{}, "ArgocdResources"),
	)
}

// boolPtrEqualEffective returns true if two `*bool` values represent
// the same effective boolean (nil == &false; &true == &true).
func boolPtrEqualEffective(a, b *bool) bool {
	aVal := a != nil && *a
	bVal := b != nil && *b
	return aVal == bVal
}

// resolvedSecret captures the name of the referenced kube Secret
// alongside its resolved key/value data. Carrying the name into the
// digest means a user who renames the reference (from Secret "A" to
// Secret "B" with identical content) still gets a drift signal — the
// controller can't otherwise tell that the underlying chain of trust
// changed. An empty Name marks an unset reference.
type resolvedSecret struct {
	Name string
	Data map[string]string
}

// resolvedNamedSecret is the equivalent for NamedLocalSecretReference
// entries (repo credentials): the gateway-facing slot Name is already
// meaningful, but the underlying kube Secret's name must also
// contribute to the digest.
type resolvedNamedSecret struct {
	// Slot is the gateway-facing credential name (must start with
	// "repo-"). It's the map key in resolvedInstanceSecrets.
	Slot string
	// SecretName is the name of the referenced kube Secret.
	SecretName string
	// Data is the resolved key/value payload.
	Data map[string]string
}

// resolvedInstanceSecrets bundles all sensitive payloads a user may
// attach to an Instance spec, pre-loaded by the controller from kube
// Secrets. Each field is populated by resolveInstanceSecrets and
// consumed by buildApplyRequest; keeping resolution out of the request
// builder preserves its purity for unit tests.
type resolvedInstanceSecrets struct {
	ArgoCD            resolvedSecret
	Notifications     resolvedSecret
	ImageUpdater      resolvedSecret
	ApplicationSet    resolvedSecret
	RepoCredentials   []resolvedNamedSecret
	RepoTemplateCreds []resolvedNamedSecret
}

// resolveInstanceSecrets loads every Secret referenced by the Instance's
// spec.forProvider and returns the flattened data maps paired with the
// referenced Secret name. Empty/missing refs yield zero resolvedSecret
// values, which the downstream wire builder treats as "field omitted".
// All lookups run in the MR's own namespace.
func resolveInstanceSecrets(ctx context.Context, kube client.Client, mg *v1alpha2.Instance) (resolvedInstanceSecrets, error) {
	ns := mg.GetNamespace()
	p := &mg.Spec.ForProvider
	out := resolvedInstanceSecrets{}

	var err error
	if out.ArgoCD, err = resolveOne(ctx, kube, ns, p.ArgoCDSecretRef); err != nil {
		return out, fmt.Errorf("argocdSecretRef: %w", err)
	}
	if out.Notifications, err = resolveOne(ctx, kube, ns, p.ArgoCDNotificationsSecretRef); err != nil {
		return out, fmt.Errorf("argocdNotificationsSecretRef: %w", err)
	}
	if out.ImageUpdater, err = resolveOne(ctx, kube, ns, p.ArgoCDImageUpdaterSecretRef); err != nil {
		return out, fmt.Errorf("argocdImageUpdaterSecretRef: %w", err)
	}
	if out.ApplicationSet, err = resolveOne(ctx, kube, ns, p.ApplicationSetSecretRef); err != nil {
		return out, fmt.Errorf("applicationSetSecretRef: %w", err)
	}
	if out.RepoCredentials, err = resolveNamed(ctx, kube, ns, p.RepoCredentialSecretRefs); err != nil {
		return out, fmt.Errorf("repoCredentialSecretRefs: %w", err)
	}
	if out.RepoTemplateCreds, err = resolveNamed(ctx, kube, ns, p.RepoTemplateCredentialSecretRefs); err != nil {
		return out, fmt.Errorf("repoTemplateCredentialSecretRefs: %w", err)
	}
	return out, nil
}

func resolveOne(ctx context.Context, kube client.Client, ns string, ref *xpv1.LocalSecretReference) (resolvedSecret, error) {
	data, err := secrets.ResolveAllKeys(ctx, kube, ns, ref)
	if err != nil {
		return resolvedSecret{}, err
	}
	if ref == nil {
		return resolvedSecret{}, nil
	}
	return resolvedSecret{Name: ref.Name, Data: data}, nil
}

func resolveNamed(ctx context.Context, kube client.Client, ns string, refs []v1alpha2.NamedLocalSecretReference) ([]resolvedNamedSecret, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	out := make([]resolvedNamedSecret, 0, len(refs))
	seen := map[string]struct{}{}
	for i := range refs {
		slot := refs[i].Name
		if _, dup := seen[slot]; dup {
			return nil, fmt.Errorf("duplicate slot %q", slot)
		}
		seen[slot] = struct{}{}
		data, err := secrets.ResolveAllKeys(ctx, kube, ns, &refs[i].SecretRef)
		if err != nil {
			return nil, fmt.Errorf("slot %q: %w", slot, err)
		}
		out = append(out, resolvedNamedSecret{
			Slot:       slot,
			SecretName: refs[i].SecretRef.Name,
			Data:       data,
		})
	}
	return out, nil
}

// Hash combines the digests of every resolved secret plus their
// referenced Secret names into a single stable string. A change in
// any of (ref.Name, underlying content, slot name) yields a different
// hash, so both a Secret rotation and a rename to a different Secret
// containing identical bytes force a re-Apply on the next reconcile.
func (r resolvedInstanceSecrets) Hash() string {
	parts := map[string]string{
		"argocd":         hashOne(r.ArgoCD),
		"notifications":  hashOne(r.Notifications),
		"imageUpdater":   hashOne(r.ImageUpdater),
		"applicationSet": hashOne(r.ApplicationSet),
		"repoCreds":      hashNamed(r.RepoCredentials),
		"repoTmplCreds":  hashNamed(r.RepoTemplateCreds),
	}
	empty := true
	for _, p := range parts {
		if p != "" {
			empty = false
			break
		}
	}
	if empty {
		return ""
	}
	return secrets.Hash(parts)
}

// hashOne mixes the referenced Secret's name into the content digest
// so a rename-with-identical-content yields a different result.
func hashOne(s resolvedSecret) string {
	if s.Name == "" && len(s.Data) == 0 {
		return ""
	}
	return secrets.Hash(map[string]string{
		"__ref__":  s.Name,
		"__data__": secrets.Hash(s.Data),
	})
}

// hashNamed mixes the slot name + underlying Secret name + content
// digest for every named credential. A collision requires identical
// slot list, identical backing Secret names, and identical byte
// content across every slot.
func hashNamed(entries []resolvedNamedSecret) string {
	if len(entries) == 0 {
		return ""
	}
	per := make(map[string]string, len(entries))
	for _, e := range entries {
		per[e.Slot] = secrets.Hash(map[string]string{
			"__secret__": e.SecretName,
			"__data__":   secrets.Hash(e.Data),
		})
	}
	return secrets.Hash(per)
}

// buildApplyRequest materialises an ApplyInstanceRequest targeting the
// Akuity API. Non-ConfigMap payloads flow through the generated
// converters; ConfigMaps and plugins are marshalled directly via the
// existing protobuf helper. Sensitive payloads are provided pre-resolved
// via resolvedInstanceSecrets so this function stays independent of the
// kube client.
//
//nolint:gocyclo // buildApplyRequest assembles 7 ConfigMaps + 6 secrets + plugins + 3 declarative-child slots into a single proto; the linear flow is the clearest form.
func buildApplyRequest(_ context.Context, ac akuity.Client, mg *v1alpha2.Instance, sec resolvedInstanceSecrets) (*argocdv1.ApplyInstanceRequest, error) {
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

	argocdSecretPB, err := secretToPB(argoCDSecretKey, sec.ArgoCD.Data, nil)
	if err != nil {
		return nil, err
	}
	notificationsSecretPB, err := secretToPB(argoCDNotificationsSecretKey, sec.Notifications.Data, nil)
	if err != nil {
		return nil, err
	}
	imageUpdaterSecretPB, err := secretToPB(argoCDImageUpdaterSecretKey, sec.ImageUpdater.Data, nil)
	if err != nil {
		return nil, err
	}
	appSetSecretPB, err := secretToPB(applicationSetSecretKey, sec.ApplicationSet.Data, nil)
	if err != nil {
		return nil, err
	}
	repoCredsPB, err := namedSecretsToPB(sec.RepoCredentials, map[string]string{secretTypeLabel: secretTypeRepository})
	if err != nil {
		return nil, err
	}
	repoTemplateCredsPB, err := namedSecretsToPB(sec.RepoTemplateCreds, map[string]string{secretTypeLabel: secretTypeRepoCreds})
	if err != nil {
		return nil, err
	}

	apps, appSets, appProjects, err := splitArgocdResources(p.ArgocdResources)
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

		ArgocdSecret:                  argocdSecretPB,
		NotificationsSecret:           notificationsSecretPB,
		ImageUpdaterSecret:            imageUpdaterSecretPB,
		ApplicationSetSecret:          appSetSecretPB,
		RepoCredentialSecrets:         repoCredsPB,
		RepoTemplateCredentialSecrets: repoTemplateCredsPB,

		Applications:    apps,
		ApplicationSets: appSets,
		AppProjects:     appProjects,

		// Mirror the conservative pruning posture used by other Akuity
		// clients: allow the gateway to GC config-management plugins
		// the user has removed from spec, but leave the heavier-impact
		// categories (Applications, Clusters, AppProjects, ...) in place
		// so out-of-band state survives a spec edit.
		PruneResourceTypes: []argocdv1.PruneResourceType{
			argocdv1.PruneResourceType_PRUNE_RESOURCE_TYPE_CONFIG_MANAGEMENT_PLUGINS,
		},
	}, nil
}

// argocdResourcesUpToDate reports whether every child declared in
// spec.forProvider.argocdResources is present on the gateway with an
// equivalent payload. Removal of an entry from spec does NOT trigger
// server-side deletion — the provider runs with additive semantics
// around declarative children to avoid clobbering resources managed by
// the Akuity platform UI or other tooling. Drift is therefore a one-
// way signal: every desired child must be reflected server-side.
//
// Returns (true, nil) when desired ⊆ observed; (false, nil) when
// something needs to be re-Applied; error when the desired bundle is
// malformed. Server-response decoding errors fall back to "up to
// date" with a log line rather than failing the reconcile loop — the
// Apply path will resurface any wire-format breakage.
func argocdResourcesUpToDate(desired []runtime.RawExtension, exp *argocdv1.ExportInstanceResponse) (bool, children.DriftReport, error) {
	if len(desired) == 0 {
		return true, children.DriftReport{}, nil
	}
	desiredIdx, err := children.Index(desired)
	if err != nil {
		return false, children.DriftReport{}, fmt.Errorf("argocdResources: %w", err)
	}
	// Combine Applications/ApplicationSets/AppProjects into a single
	// observed index. Identity carries apiVersion + kind so there is
	// no chance of two different server-side kinds colliding on name.
	observedAll := make(map[children.Identity]map[string]interface{})
	for _, group := range [][]*structpb.Struct{
		exp.GetApplications(),
		exp.GetApplicationSets(),
		exp.GetAppProjects(),
	} {
		group := group
		idx, err := children.IndexStructs(group)
		if err != nil {
			// Log-worthy but not fatal — the next Apply will catch
			// whatever is wrong. Treating this as "drift" would force
			// a re-Apply storm for a transient decode glitch.
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

// splitArgocdResources validates each spec.forProvider.argocdResources
// entry, enforces the argoproj.io/v1alpha1 kind allowlist, and returns
// the per-kind slices of structpb.Structs expected by
// ApplyInstanceRequest.{Applications, ApplicationSets, AppProjects}.
// Empty input yields three nil slices and no error so callers can
// compose the result without pre-checks.
func splitArgocdResources(in []runtime.RawExtension) (apps, appSets, appProjects []*structpb.Struct, err error) {
	if len(in) == 0 {
		return nil, nil, nil, nil
	}
	for i, raw := range in {
		if len(raw.Raw) == 0 {
			// Empty entries are meaningless in a typed spec; fail loudly
			// so the user catches a bad YAML snippet at reconcile time
			// instead of it being silently dropped.
			return nil, nil, nil, fmt.Errorf("argocdResources[%d]: empty payload", i)
		}
		obj := map[string]interface{}{}
		if err := json.Unmarshal(raw.Raw, &obj); err != nil {
			return nil, nil, nil, fmt.Errorf("argocdResources[%d]: invalid JSON: %w", i, err)
		}
		apiVersion, _ := obj["apiVersion"].(string)
		kind, _ := obj["kind"].(string)
		if apiVersion != argoprojAPIVersion {
			return nil, nil, nil, fmt.Errorf("argocdResources[%d]: unsupported apiVersion %q (want %s)", i, apiVersion, argoprojAPIVersion)
		}
		pb, err := structpb.NewStruct(obj)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("argocdResources[%d]: structpb encode: %w", i, err)
		}
		switch kind {
		case argoKindApplication:
			apps = append(apps, pb)
		case argoKindApplicationSet:
			appSets = append(appSets, pb)
		case argoKindAppProject:
			appProjects = append(appProjects, pb)
		default:
			return nil, nil, nil, fmt.Errorf("argocdResources[%d]: unsupported kind %q (want one of %s, %s, %s)",
				i, kind, argoKindApplication, argoKindApplicationSet, argoKindAppProject)
		}
	}
	return apps, appSets, appProjects, nil
}

// secretToPB marshals a resolved secret map into a Kubernetes Secret
// structpb.Struct, labelled as requested. Returns nil with no error
// when data is empty so callers can compose it without pre-checks.
func secretToPB(name string, data map[string]string, labels map[string]string) (*structpb.Struct, error) {
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
	pb, err := protobuf.MarshalObjectToProtobufStruct(sec)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s secret to protobuf: %w", name, err)
	}
	return pb, nil
}

// namedSecretsToPB emits a []*structpb.Struct where each entry is a
// Kubernetes Secret named after its slot (the gateway-facing
// credential identifier). Labels are applied to every entry. Order is
// stable (sorted by slot) so the Apply request is byte-identical
// across reconciles for the same input.
func namedSecretsToPB(in []resolvedNamedSecret, labels map[string]string) ([]*structpb.Struct, error) {
	if len(in) == 0 {
		return nil, nil
	}
	// Input arrives in spec order; sort by slot for byte-stability.
	sorted := append([]resolvedNamedSecret(nil), in...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slot < sorted[j].Slot })

	out := make([]*structpb.Struct, 0, len(sorted))
	for _, e := range sorted {
		pb, err := secretToPB(e.Slot, e.Data, labels)
		if err != nil {
			return nil, err
		}
		if pb != nil {
			out = append(out, pb)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
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
