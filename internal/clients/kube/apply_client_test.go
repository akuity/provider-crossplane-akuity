package kube_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	dynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/akuityio/provider-crossplane-akuity/internal/clients/kube"
)

var (
	ctx = context.TODO()
)

func TestApplyClient_EmptyManifests(t *testing.T) {
	applyClient, err := kube.NewApplyClient(dynamic.NewSimpleDynamicClient(scheme.Scheme), fake.NewSimpleClientset())
	require.NoError(t, err)

	err = applyClient.ApplyManifests(ctx, "", false)
	require.NoError(t, err)
}

func TestApplyClient_ApplyManifestsInvalidKindErr(t *testing.T) {
	applyClient, err := kube.NewApplyClient(dynamic.NewSimpleDynamicClient(scheme.Scheme), fake.NewSimpleClientset())
	require.NoError(t, err)

	err = applyClient.ApplyManifests(ctx, "apiVersion: v1\nkind: InvalidKind\nmetadata:\n  name: test-pod", false)
	require.Error(t, err)
}
