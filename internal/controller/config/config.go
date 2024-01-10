/*
Copyright 2020 The Crossplane Authors.

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
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/providerconfig"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
)

const (
	DefaultAkuityClientServerURL = "https://akuity.cloud/"
	CredentialsAPIKeyID          = "apiKeyId"
	CredentialsAPIKeySecret      = "apiKeySecret"
)

// Setup adds a controller that reconciles ProviderConfigs by accounting for
// their current usage.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := providerconfig.ControllerName(apisv1alpha1.ProviderConfigGroupKind)

	of := resource.ProviderConfigKinds{
		Config:    apisv1alpha1.ProviderConfigGroupVersionKind,
		UsageList: apisv1alpha1.ProviderConfigUsageListGroupVersionKind,
	}

	r := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
		providerconfig.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&apisv1alpha1.ProviderConfig{}).
		Watches(&apisv1alpha1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

func GetAkuityClientFromProviderConfig(ctx context.Context, kubeClient client.Client, providerConfigName string) (akuity.Client, error) {
	providerConfig := &apisv1alpha1.ProviderConfig{}
	if err := kubeClient.Get(ctx, k8stypes.NamespacedName{Name: providerConfigName}, providerConfig); err != nil {
		return nil, err
	}

	secret := &corev1.Secret{}
	if err := kubeClient.Get(ctx, k8stypes.NamespacedName{Name: providerConfig.Spec.CredentialsSecretRef.Name, Namespace: providerConfig.Spec.CredentialsSecretRef.Namespace}, secret); err != nil {
		return nil, fmt.Errorf("could not get secret from the Kubernetes API: %w", err)
	}

	secretData := make(map[string]string)
	if err := json.Unmarshal(secret.Data[providerConfig.Spec.CredentialsSecretRef.Key], &secretData); err != nil {
		return nil, fmt.Errorf("could not unmarshal secret data: %w", err)
	}

	gatewayClient := argocdv1.NewArgoCDServiceGatewayClient(gwoption.NewClient(getAkuityClientServerURL(providerConfig.Spec.ServerURL), providerConfig.Spec.SkipTLSVerify))
	akuityClient, err := akuity.NewClient(providerConfig.Spec.OrganizationID, secretData[CredentialsAPIKeyID], secretData[CredentialsAPIKeySecret], gatewayClient)
	if err != nil {
		return nil, fmt.Errorf("cannot create Akuity client: %w", err)
	}

	return akuityClient, nil
}

func getAkuityClientServerURL(serverURL string) string {
	if serverURL == "" {
		return DefaultAkuityClientServerURL
	}

	return serverURL
}
