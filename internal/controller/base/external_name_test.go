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

package base

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
)

func kargoAgentSpecName(mg *v1alpha1.KargoAgent) string {
	return mg.Spec.ForProvider.Name
}

func newNamedKargoAgent(specName string) *v1alpha1.KargoAgent {
	a := &v1alpha1.KargoAgent{ObjectMeta: metav1.ObjectMeta{Name: "managed-agt"}}
	a.Spec.ForProvider.Name = specName
	return a
}

func newExternalNameKube(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.SchemeBuilder.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestSpecNameAsExternalName_SetsSpecName(t *testing.T) {
	a := newNamedKargoAgent("agt")
	kube := newExternalNameKube(t, a)

	require.NoError(t, SpecNameAsExternalName(kube, kargoAgentSpecName)(context.Background(), a))

	assert.Equal(t, "agt", meta.GetExternalName(a))
	stored := &v1alpha1.KargoAgent{}
	require.NoError(t, kube.Get(context.Background(), client.ObjectKeyFromObject(a), stored))
	assert.Equal(t, "agt", meta.GetExternalName(stored))
}

// TestSpecNameAsExternalName_OverwritesMetadataName covers a resource the
// default NameAsExternalName initializer already stamped with
// metadata.name before its first successful Create.
func TestSpecNameAsExternalName_OverwritesMetadataName(t *testing.T) {
	a := newNamedKargoAgent("agt")
	meta.SetExternalName(a, "managed-agt")
	kube := newExternalNameKube(t, a)

	require.NoError(t, SpecNameAsExternalName(kube, kargoAgentSpecName)(context.Background(), a))

	stored := &v1alpha1.KargoAgent{}
	require.NoError(t, kube.Get(context.Background(), client.ObjectKeyFromObject(a), stored))
	assert.Equal(t, "agt", meta.GetExternalName(stored))
}

// TestSpecNameAsExternalName_MatchingNameSkipsUpdate passes a resource
// the client does not know, so any Update would fail.
func TestSpecNameAsExternalName_MatchingNameSkipsUpdate(t *testing.T) {
	a := newNamedKargoAgent("agt")
	meta.SetExternalName(a, "agt")

	require.NoError(t, SpecNameAsExternalName(newExternalNameKube(t), kargoAgentSpecName)(context.Background(), a))
	assert.Equal(t, "agt", meta.GetExternalName(a))
}

func TestSpecNameAsExternalName_EmptySpecNameSkipsUpdate(t *testing.T) {
	a := newNamedKargoAgent("")

	require.NoError(t, SpecNameAsExternalName(newExternalNameKube(t), kargoAgentSpecName)(context.Background(), a))
	assert.Empty(t, meta.GetExternalName(a))
}

func TestSpecNameAsExternalName_UnexpectedTypeFails(t *testing.T) {
	c := &v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "managed-cluster"}}

	err := SpecNameAsExternalName(newExternalNameKube(t), kargoAgentSpecName)(context.Background(), c)
	require.Error(t, err)
	assert.Empty(t, meta.GetExternalName(c))
}
