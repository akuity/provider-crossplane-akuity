package base

import (
	"context"
	"errors"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	apisv1alpha2 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
)

func TestConnectorPropagatesObservedGenerationWhenNewClientFails(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1alpha2.SchemeBuilder.AddToScheme(scheme))
	require.NoError(t, apisv1alpha2.SchemeBuilder.AddToScheme(scheme))

	kube := fake.NewClientBuilder().WithScheme(scheme).Build()
	conn := &Connector[*corev1alpha2.Instance]{
		Kube:   kube,
		Usage:  resource.NewProviderConfigUsageTracker(kube, &apisv1alpha2.ProviderConfigUsage{}),
		Logger: logging.NewNopLogger(),
		NewClient: func(context.Context, client.Client, resource.ModernManaged) (akuity.Client, error) {
			return nil, errors.New("boom")
		},
	}

	mg := &corev1alpha2.Instance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1alpha2.SchemeGroupVersion.String(),
			Kind:       corev1alpha2.InstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "inst",
			Namespace:  "ns",
			UID:        types.UID("uid-1"),
			Generation: 42,
		},
	}
	mg.Spec.ProviderConfigReference = &xpv1.ProviderConfigReference{
		Name: "default",
		Kind: apisv1alpha2.ProviderConfigKind,
	}

	_, err := conn.Connect(context.Background(), mg)
	require.ErrorContains(t, err, "boom")
	require.Equal(t, int64(42), mg.Status.ObservedGeneration)
}

func TestConnectorPropagatesObservedGenerationWhenUsageTrackingFails(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1alpha2.SchemeBuilder.AddToScheme(scheme))
	require.NoError(t, apisv1alpha2.SchemeBuilder.AddToScheme(scheme))

	kube := fake.NewClientBuilder().WithScheme(scheme).Build()
	conn := &Connector[*corev1alpha2.Instance]{
		Kube:   kube,
		Usage:  resource.NewProviderConfigUsageTracker(kube, &apisv1alpha2.ProviderConfigUsage{}),
		Logger: logging.NewNopLogger(),
		NewClient: func(context.Context, client.Client, resource.ModernManaged) (akuity.Client, error) {
			t.Fatal("NewClient should not run when usage tracking fails")
			return nil, nil
		},
	}

	mg := &corev1alpha2.Instance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1alpha2.SchemeGroupVersion.String(),
			Kind:       corev1alpha2.InstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "inst",
			Namespace:  "ns",
			UID:        types.UID("uid-2"),
			Generation: 99,
		},
	}

	_, err := conn.Connect(context.Background(), mg)
	require.Error(t, err)
	require.Equal(t, int64(99), mg.Status.ObservedGeneration)
}
