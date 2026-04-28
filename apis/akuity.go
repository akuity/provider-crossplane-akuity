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

// Package apis registers the Kubernetes APIs served by the Akuity provider.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	akuityv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
)

func init() {
	AddToSchemes = append(AddToSchemes,
		akuityv1alpha1.SchemeBuilder.AddToScheme,
		corev1alpha1.SchemeBuilder.AddToScheme,
	)
}

// AddToSchemes adds all project resources to a Scheme.
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all project resources to the supplied Scheme.
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
