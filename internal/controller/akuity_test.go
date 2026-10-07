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

package controller_test

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/akuityio/provider-crossplane-akuity/apis"
	akuity "github.com/akuityio/provider-crossplane-akuity/internal/controller"
)

type recordingGate struct {
	registered []schema.GroupVersionKind
}

func (g *recordingGate) Register(_ func(), gvks ...schema.GroupVersionKind) {
	g.registered = append(g.registered, gvks...)
}

func (g *recordingGate) Set(schema.GroupVersionKind, bool) bool { return false }

// Every managed and ProviderConfig kind in the scheme must be gated.
func TestSetupGated_RegistersEveryKind(t *testing.T) {
	s := runtime.NewScheme()
	require.NoError(t, apis.AddToScheme(s))

	var want []schema.GroupVersionKind
	for gvk := range s.AllKnownTypes() {
		obj, err := s.New(gvk)
		require.NoError(t, err)
		switch obj.(type) {
		case resource.Managed, resource.ProviderConfig, resource.ProviderConfigUsage:
			want = append(want, gvk)
		}
	}
	require.NotEmpty(t, want)

	g := &recordingGate{}
	require.NoError(t, akuity.SetupGated(nil, controller.Options{Gate: g}))
	assert.ElementsMatch(t, want, g.registered)
}
