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

package glue

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestKustomizationRoundtrip(t *testing.T) {
	in := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nnamespace: foo\n"
	raw := KustomizationStringToRaw(in)
	if len(raw.Raw) == 0 {
		t.Fatalf("raw is empty after encode")
	}
	out := KustomizationRawToString(raw)
	if !strings.Contains(out, "kind: Kustomization") {
		t.Fatalf("round-trip lost kind marker: %q", out)
	}
	if !strings.Contains(out, "namespace: foo") {
		t.Fatalf("round-trip lost namespace: %q", out)
	}
}

func TestKustomizationEmpty(t *testing.T) {
	if raw := KustomizationStringToRaw(""); len(raw.Raw) != 0 {
		t.Fatalf("empty string should yield empty RawExtension, got %q", raw.Raw)
	}
	if got := KustomizationRawToString(runtime.RawExtension{}); got != "" {
		t.Fatalf("empty RawExtension should yield empty string, got %q", got)
	}
}
