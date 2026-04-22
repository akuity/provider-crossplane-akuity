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

func TestDexConfigSecretResolvedToAPI_EmptyReturnsNil(t *testing.T) {
	if got := DexConfigSecretResolvedToAPI(nil); got != nil {
		t.Fatalf("nil input: got=%v, want nil", got)
	}
	if got := DexConfigSecretResolvedToAPI(map[string]string{}); got != nil {
		t.Fatalf("empty input: got=%v, want nil", got)
	}
}

func TestDexConfigSecretResolvedToAPI_WrapsEachValue(t *testing.T) {
	in := map[string]string{
		"github-client-secret": "gh-abc",
		"google-client-secret": "goog-def",
	}
	got := DexConfigSecretResolvedToAPI(in)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	for k, want := range in {
		v, ok := got[k]
		if !ok {
			t.Fatalf("missing key %q", k)
		}
		if v.Value == nil {
			t.Fatalf("key %q: Value is nil, want &%q", k, want)
		}
		if *v.Value != want {
			t.Fatalf("key %q: got %q, want %q", k, *v.Value, want)
		}
	}
}

func TestDexConfigSecretResolvedToAPI_PointerCapture(t *testing.T) {
	// Go's range-by-value capture is a classic bug vector. Ensure each
	// entry in the output keeps a pointer to its own value, not to the
	// last iteration's loop variable.
	in := map[string]string{"a": "1", "b": "2", "c": "3"}
	got := DexConfigSecretResolvedToAPI(in)
	for k, want := range in {
		if got[k].Value == nil || *got[k].Value != want {
			t.Fatalf("key %q: got %v, want %q", k, got[k].Value, want)
		}
	}
}

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
