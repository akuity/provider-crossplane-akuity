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

package config

import (
	"context"
	"encoding/json"
	"fmt"

	gwoption "github.com/akuity/api-client-go/pkg/api/gateway/option"
	argocdv1 "github.com/akuity/api-client-go/pkg/api/gen/argocd/v1"
	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/providerconfig"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apisv1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/event"
)

// SetupV2 registers the namespaced ProviderConfig reconciler and the
// cluster-scoped ClusterProviderConfig reconciler. Both share a single
// v1alpha2 ProviderConfigUsage list for usage bookkeeping; usage
// records disambiguate between PC kinds via the Kind on the embedded
// ProviderConfigReference.
func SetupV2(mgr ctrl.Manager, o controller.Options) error {
	if err := setupProviderConfigV2(mgr, o); err != nil {
		return err
	}
	return setupClusterProviderConfigV2(mgr, o)
}

func setupProviderConfigV2(mgr ctrl.Manager, o controller.Options) error {
	name := providerconfig.ControllerName(apisv1alpha2.ProviderConfigGroupKind)
	of := resource.ProviderConfigKinds{
		Config:    apisv1alpha2.ProviderConfigGroupVersionKind,
		Usage:     apisv1alpha2.ProviderConfigUsageGroupVersionKind,
		UsageList: apisv1alpha2.ProviderConfigUsageListGroupVersionKind,
	}
	r := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
		providerconfig.WithRecorder(event.NewRecorder(mgr, name)))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&apisv1alpha2.ProviderConfig{}).
		Watches(&apisv1alpha2.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

func setupClusterProviderConfigV2(mgr ctrl.Manager, o controller.Options) error {
	name := providerconfig.ControllerName(apisv1alpha2.ClusterProviderConfigGroupKind)
	of := resource.ProviderConfigKinds{
		Config:    apisv1alpha2.ClusterProviderConfigGroupVersionKind,
		Usage:     apisv1alpha2.ProviderConfigUsageGroupVersionKind,
		UsageList: apisv1alpha2.ProviderConfigUsageListGroupVersionKind,
	}
	r := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
		providerconfig.WithRecorder(event.NewRecorder(mgr, name)))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&apisv1alpha2.ClusterProviderConfig{}).
		Watches(&apisv1alpha2.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// GetAkuityClientFromV2Ref builds an Akuity client from a v1alpha2
// managed resource's ProviderConfigReference. It supports both the
// namespaced ProviderConfig (resolved in mrNamespace; credentials
// secret assumed in the same namespace) and the cluster-scoped
// ClusterProviderConfig (resolved cluster-wide; credentials secret
// takes an explicit namespace from the PC spec).
func GetAkuityClientFromV2Ref(ctx context.Context, kubeClient client.Client, ref *xpv1.ProviderConfigReference, mrNamespace string) (akuity.Client, error) {
	if ref == nil {
		return nil, fmt.Errorf("managed resource has no ProviderConfigReference")
	}

	switch ref.Kind {
	case apisv1alpha2.ProviderConfigKind, "":
		return getFromProviderConfigV2(ctx, kubeClient, ref.Name, mrNamespace)
	case apisv1alpha2.ClusterProviderConfigKind:
		return getFromClusterProviderConfigV2(ctx, kubeClient, ref.Name)
	default:
		return nil, fmt.Errorf("unsupported ProviderConfig kind %q (expected %q or %q)",
			ref.Kind, apisv1alpha2.ProviderConfigKind, apisv1alpha2.ClusterProviderConfigKind)
	}
}

func getFromProviderConfigV2(ctx context.Context, kubeClient client.Client, name, mrNamespace string) (akuity.Client, error) {
	pc := &apisv1alpha2.ProviderConfig{}
	if err := kubeClient.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: mrNamespace}, pc); err != nil {
		return nil, fmt.Errorf("could not get ProviderConfig %s/%s: %w", mrNamespace, name, err)
	}

	secret := &corev1.Secret{}
	if err := kubeClient.Get(ctx, k8stypes.NamespacedName{
		Name:      pc.Spec.CredentialsSecretRef.Name,
		Namespace: mrNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("could not get credentials secret %s/%s: %w", mrNamespace, pc.Spec.CredentialsSecretRef.Name, err)
	}

	return buildAkuityClient(pc.Spec.OrganizationID, pc.Spec.ServerURL, pc.Spec.SkipTLSVerify,
		secret.Data[pc.Spec.CredentialsSecretRef.Key])
}

func getFromClusterProviderConfigV2(ctx context.Context, kubeClient client.Client, name string) (akuity.Client, error) {
	cpc := &apisv1alpha2.ClusterProviderConfig{}
	if err := kubeClient.Get(ctx, k8stypes.NamespacedName{Name: name}, cpc); err != nil {
		return nil, fmt.Errorf("could not get ClusterProviderConfig %s: %w", name, err)
	}

	secret := &corev1.Secret{}
	if err := kubeClient.Get(ctx, k8stypes.NamespacedName{
		Name:      cpc.Spec.CredentialsSecretRef.Name,
		Namespace: cpc.Spec.CredentialsSecretRef.Namespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("could not get credentials secret %s/%s: %w",
			cpc.Spec.CredentialsSecretRef.Namespace, cpc.Spec.CredentialsSecretRef.Name, err)
	}

	return buildAkuityClient(cpc.Spec.OrganizationID, cpc.Spec.ServerURL, cpc.Spec.SkipTLSVerify,
		secret.Data[cpc.Spec.CredentialsSecretRef.Key])
}

func buildAkuityClient(orgID, serverURL string, skipTLS bool, credBytes []byte) (akuity.Client, error) {
	secretData := make(map[string]string)
	if err := json.Unmarshal(credBytes, &secretData); err != nil {
		return nil, fmt.Errorf("could not unmarshal secret data: %w", err)
	}

	gwClient := gwoption.NewClient(getAkuityClientServerURL(serverURL), skipTLS)
	argoGW := argocdv1.NewArgoCDServiceGatewayClient(gwClient)
	kargoGW := kargov1.NewKargoServiceGatewayClient(gwClient)
	ac, err := akuity.NewClient(orgID, secretData[CredentialsAPIKeyID], secretData[CredentialsAPIKeySecret], argoGW, kargoGW)
	if err != nil {
		return nil, fmt.Errorf("cannot create Akuity client: %w", err)
	}
	return ac, nil
}
