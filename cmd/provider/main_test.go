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

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
)

// The manager scheme is built by hand rather than from client-go's full
// scheme, so every kind the provider sends through the manager client
// must be registered explicitly, including the meta types the typed
// client encodes list and create options with.
func TestNewScheme(t *testing.T) {
	s, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme(): %v", err)
	}

	cases := map[string]struct {
		gvk  schema.GroupVersionKind
		want bool
	}{
		"Secret":                   {gvk: corev1.SchemeGroupVersion.WithKind("Secret"), want: true},
		"SecretList":               {gvk: corev1.SchemeGroupVersion.WithKind("SecretList"), want: true},
		"CoreListOptions":          {gvk: corev1.SchemeGroupVersion.WithKind("ListOptions"), want: true},
		"SelfSubjectAccessReview":  {gvk: authv1.SchemeGroupVersion.WithKind("SelfSubjectAccessReview"), want: true},
		"AuthCreateOptions":        {gvk: authv1.SchemeGroupVersion.WithKind("CreateOptions"), want: true},
		"CustomResourceDefinition": {gvk: apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), want: true},
		"ProviderConfig":           {gvk: apisv1alpha1.ProviderConfigGroupVersionKind, want: true},
		"ProviderConfigUsage":      {gvk: apisv1alpha1.ProviderConfigUsageGroupVersionKind, want: true},
		"Instance":                 {gvk: corev1alpha1.InstanceGroupVersionKind, want: true},
		"InstanceList":             {gvk: corev1alpha1.SchemeGroupVersion.WithKind("InstanceList"), want: true},
		"ConfigMapUnregistered":    {gvk: corev1.SchemeGroupVersion.WithKind("ConfigMap"), want: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, s.Recognizes(tc.gvk)); diff != "" {
				t.Errorf("Recognizes(%s): -want, +got:\n%s", tc.gvk, diff)
			}
		})
	}
}
