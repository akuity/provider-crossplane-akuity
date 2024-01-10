package config_test

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/config"
)

var (
	providerConfig = &apisv1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-provider-config",
		},
		Spec: apisv1alpha1.ProviderConfigSpec{
			CredentialsSecretRef: xpv1.SecretKeySelector{
				SecretReference: xpv1.SecretReference{
					Name:      "test-secret",
					Namespace: "test-namespace",
				},
				Key: "test-key",
			},
			OrganizationID: "test-org-id",
			ServerURL:      "test-server-url",
			SkipTLSVerify:  true,
		},
	}

	providerConfigSecret = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"test-key": []byte(`{"` + config.CredentialsAPIKeyID + `": "test-api-key-id", "` + config.CredentialsAPIKeySecret + `": "test-api-key-secret"}`),
		},
	}
)

func TestGetAkuityClientFromProviderConfig_ProviderConfigNotFoundErr(t *testing.T) {
	s := scheme.Scheme
	apisv1alpha1.SchemeBuilder.AddToScheme(s)

	kube := fake.NewClientBuilder().WithScheme(s).Build()

	_, err := config.GetAkuityClientFromProviderConfig(context.TODO(), kube, "test-provider-config")
	require.Error(t, err)
}

func TestGetAkuityClientFromProviderConfig_ProviderConfigSecretNotFoundErr(t *testing.T) {
	s := scheme.Scheme
	apisv1alpha1.SchemeBuilder.AddToScheme(s)

	kube := fake.NewClientBuilder().WithScheme(s).
		WithRuntimeObjects(providerConfig).Build()

	_, err := config.GetAkuityClientFromProviderConfig(context.TODO(), kube, "test-provider-config")
	require.Error(t, err)
}

func TestGetAkuityClientFromProviderConfig_UnmarshalProviderConfigSecretErr(t *testing.T) {
	providerConfigSecretCopy := providerConfigSecret.DeepCopy()
	providerConfigSecretCopy.Data[providerConfig.Spec.CredentialsSecretRef.Key] = []byte("{{{{}")

	s := scheme.Scheme
	apisv1alpha1.SchemeBuilder.AddToScheme(s)

	kube := fake.NewClientBuilder().WithScheme(s).
		WithRuntimeObjects(providerConfig, providerConfigSecretCopy).Build()

	_, err := config.GetAkuityClientFromProviderConfig(context.TODO(), kube, "test-provider-config")
	require.Error(t, err)
}

func TestGetAkuityClientFromProviderConfig_CreateAkuityClientErr(t *testing.T) {
	s := scheme.Scheme
	apisv1alpha1.SchemeBuilder.AddToScheme(s)

	providerConfigCopy := providerConfig.DeepCopy()
	providerConfigCopy.Spec.OrganizationID = ""

	kube := fake.NewClientBuilder().WithScheme(s).
		WithRuntimeObjects(providerConfigCopy, providerConfigSecret).Build()

	_, err := config.GetAkuityClientFromProviderConfig(context.TODO(), kube, "test-provider-config")
	require.Error(t, err)
}

func TestGetAkuityClientFromProviderConfig(t *testing.T) {
	s := scheme.Scheme
	apisv1alpha1.SchemeBuilder.AddToScheme(s)

	kube := fake.NewClientBuilder().WithScheme(s).
		WithRuntimeObjects(providerConfig, providerConfigSecret).Build()

	_, err := config.GetAkuityClientFromProviderConfig(context.TODO(), kube, "test-provider-config")
	require.NoError(t, err)
}
