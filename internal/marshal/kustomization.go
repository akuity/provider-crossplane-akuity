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

package marshal

import (
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// PBStructToKustomizationYAML folds the wire-side Kustomization
// payload (a structpb.Struct the Akuity gateway ships as JSON-like
// tree) back into the YAML bytes the curated Crossplane spec stores.
// A nil / empty struct yields zero-length output so callers can
// forward the result without pre-checks.
func PBStructToKustomizationYAML(k *structpb.Struct) ([]byte, error) {
	if k == nil {
		return nil, nil
	}
	raw := runtime.RawExtension{}
	if err := RemarshalTo(k, &raw); err != nil {
		return nil, fmt.Errorf("decode kustomization struct: %w", err)
	}
	if len(raw.Raw) == 0 {
		return nil, nil
	}
	out, err := yaml.JSONToYAML(raw.Raw)
	if err != nil {
		return nil, fmt.Errorf("encode kustomization YAML: %w", err)
	}
	return out, nil
}
