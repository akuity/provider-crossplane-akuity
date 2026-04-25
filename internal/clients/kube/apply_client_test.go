package kube_test

import (
	"context"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/akuityio/provider-crossplane-akuity/internal/clients/kube"
)

var ctx = context.TODO()

func TestApplyClient_EmptyManifests(t *testing.T) {
	applyClient, err := kube.NewApplyClient(dynamic.NewSimpleDynamicClient(scheme.Scheme), fake.NewClientset(), logging.NewNopLogger())
	require.NoError(t, err)

	err = applyClient.ApplyManifests(ctx, "", false)
	require.NoError(t, err)
}

func TestApplyClient_ApplyManifestsInvalidKindErr(t *testing.T) {
	applyClient, err := kube.NewApplyClient(dynamic.NewSimpleDynamicClient(scheme.Scheme), fake.NewClientset(), logging.NewNopLogger())
	require.NoError(t, err)

	err = applyClient.ApplyManifests(ctx, "apiVersion: v1\nkind: InvalidKind\nmetadata:\n  name: test-pod", false)
	require.Error(t, err)
}

func TestApplyClient_DeleteWaitsUntilObjectGone(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "akuity"}}
	dynamicClient := dynamic.NewSimpleDynamicClient(scheme.Scheme, cm)
	dynamicClient.PrependReactor("delete", "configmaps", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		opts := action.(clientgotesting.DeleteAction).GetDeleteOptions()
		require.NotNil(t, opts.PropagationPolicy)
		require.Equal(t, metav1.DeletePropagationForeground, *opts.PropagationPolicy)
		return false, nil, nil
	})
	applyClient, err := kube.NewApplyClient(dynamicClient, fakeClientsetWithConfigMaps(), logging.NewNopLogger())
	require.NoError(t, err)

	err = applyClient.ApplyManifests(ctx, configMapManifest(), true)
	require.NoError(t, err)

	_, err = dynamicClient.Resource(configMapGVR()).Namespace("akuity").Get(ctx, "agent", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestApplyClient_DeleteErrorsWhenObjectStillExists(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "akuity"}}
	dynamicClient := dynamic.NewSimpleDynamicClient(scheme.Scheme, cm)
	dynamicClient.PrependReactor("delete", "configmaps", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})
	applyClient, err := kube.NewApplyClient(dynamicClient, fakeClientsetWithConfigMaps(), logging.NewNopLogger(), kube.WithDeleteWaitTimeout(20*time.Millisecond))
	require.NoError(t, err)

	err = applyClient.ApplyManifests(ctx, configMapManifest(), true)
	require.Error(t, err)
	require.ErrorContains(t, err, "wait for configmaps/akuity/agent to be deleted within 20ms")
}

func TestApplyClient_DeleteErrorsWhenObjectStuckOnFinalizer(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "agent",
			Namespace:  "akuity",
			Finalizers: []string{"example.com/stuck"},
		},
	}
	dynamicClient := dynamic.NewSimpleDynamicClient(scheme.Scheme, cm)
	dynamicClient.PrependReactor("delete", "configmaps", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		now := metav1.Now()
		stuck := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				Name:              "agent",
				Namespace:         "akuity",
				Finalizers:        []string{"example.com/stuck"},
				DeletionTimestamp: &now,
			},
		}
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(stuck)
		require.NoError(t, err)
		require.NoError(t, dynamicClient.Tracker().Update(configMapGVR(), &unstructured.Unstructured{Object: obj}, "akuity"))
		return true, nil, nil
	})
	applyClient, err := kube.NewApplyClient(dynamicClient, fakeClientsetWithConfigMaps(), logging.NewNopLogger(), kube.WithDeleteWaitTimeout(20*time.Millisecond))
	require.NoError(t, err)

	err = applyClient.ApplyManifests(ctx, configMapManifest(), true)
	require.Error(t, err)
	require.ErrorContains(t, err, "wait for configmaps/akuity/agent to be deleted within 20ms")
}

func configMapManifest() string {
	return "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: agent\n  namespace: akuity\n"
}

func configMapGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
}

func fakeClientsetWithConfigMaps() *fake.Clientset {
	clientset := fake.NewClientset()
	clientset.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			},
		},
	}
	return clientset
}
