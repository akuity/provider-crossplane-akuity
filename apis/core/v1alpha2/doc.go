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

// Package v1alpha2 defines managed-resource types for the Akuity
// Platform provider, served at core.m.akuity.crossplane.io.
//
// Field shapes mirror the upstream akuity-platform wire types at
// github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1
// 1:1 (same field names, same pointer-vs-value decisions) so the WS-3
// codegen tool can emit near-mechanical converters. The only blanket
// transformation is Kustomization: upstream uses
// runtime.RawExtension; v1alpha2 exposes the YAML string, converted by
// internal/convert/glue.go.
package v1alpha2
