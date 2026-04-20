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

// Package glue hosts hand-written adapters referenced by the
// codegen output. Each adapter handles a shape mismatch between the
// curated v1alpha2 types and the upstream Akuity wire types; the rest
// of the convert layer is mechanical field copying emitted from
// hack/codegen.
package glue

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// KustomizationStringToRaw parses a YAML Kustomization payload into a
// runtime.RawExtension. Empty input maps to a zero RawExtension, which
// is what the upstream Akuity API treats as "no kustomization".
//
// Errors from YAML→JSON conversion are swallowed so the generated
// converters can stay error-free; the upstream API rejects malformed
// payloads at admission, and controllers surface those as reconcile
// errors. Callers that need strict validation should call
// sigs.k8s.io/yaml.YAMLToJSON directly before handing off to the
// converter.
func KustomizationStringToRaw(s string) runtime.RawExtension {
	if s == "" {
		return runtime.RawExtension{}
	}
	raw, err := yaml.YAMLToJSON([]byte(s))
	if err != nil {
		return runtime.RawExtension{}
	}
	return runtime.RawExtension{Raw: raw}
}

// KustomizationRawToString renders a runtime.RawExtension back to YAML
// for display in a v1alpha2 CR's observed state. An empty extension
// yields an empty string. JSON→YAML conversion failures are treated
// the same way (empty string) rather than propagated because the
// converters run in observe paths where a partial status is preferable
// to bubbling up an error.
func KustomizationRawToString(r runtime.RawExtension) string {
	if len(r.Raw) == 0 {
		return ""
	}
	y, err := yaml.JSONToYAML(r.Raw)
	if err != nil {
		return ""
	}
	return string(y)
}
