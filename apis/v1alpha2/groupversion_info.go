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

// Package v1alpha2 defines provider configuration types for the Akuity
// provider, served at akuity.m.crossplane.io per the Crossplane v2
// namespaced-resource convention. The legacy cluster-scoped
// akuity.crossplane.io/v1alpha1 ProviderConfig keeps its own group and
// is unaffected.
// +kubebuilder:object:generate=true
// +groupName=akuity.m.crossplane.io
// +versionName=v1alpha2
package v1alpha2

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	Group   = "akuity.m.crossplane.io"
	Version = "v1alpha2"
)

var (
	// SchemeGroupVersion is the group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)
