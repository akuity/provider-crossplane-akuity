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

// Guard against upstream regen silently clobbering the manual additions
// transcribed into apis/core/v1alpha2/zz_generated_wire.go. These tests
// use reflection and package-level source scans rather than staring at
// the file as a string — the first catches structural regressions (a
// missing field), the second catches comment-level ones (a missing
// MANUAL ADDITIONS banner).
//
// When any of these fail, the fix is: (a) re-add the missing pieces by
// hand, AND (b) file/pursue the Follow-up Z tracked in PARITY_PLAN.md
// so the upstream gencrossplanetypes generator grows overlay support.

package v1alpha2

import (
	_ "embed"
	"reflect"
	"strings"
	"testing"
)

// wireSource is the on-disk zz_generated_wire.go content, baked into
// the test binary at compile time. Using embed rather than a
// runtime.Caller + os.ReadFile is required for the repo's
// `-trimpath`-enabled `make test`: trimpath replaces the filesystem
// path in runtime.Caller with the package's import path, which then
// cannot be opened by os.ReadFile. go:embed operates at build time
// before trimpath mangles anything, so the content is correct under
// both local `go test` and CI's `make test`.
//
//go:embed zz_generated_wire.go
var wireSource string

// TestWire_KargoOidcConfigHasDexConfigSecretRef asserts that the
// parity-added DexConfigSecretRef field on the generated KargoOidcConfig
// struct still exists. A gencrossplanetypes regen that drops it silently
// would leave users unable to declare a nested Secret reference for
// dex credentials.
func TestWire_KargoOidcConfigHasDexConfigSecretRef(t *testing.T) {
	typ := reflect.TypeOf(KargoOidcConfig{})
	f, ok := typ.FieldByName("DexConfigSecretRef")
	if !ok {
		t.Fatalf("KargoOidcConfig.DexConfigSecretRef missing — gencrossplanetypes regen likely dropped it; re-add per the MANUAL ADDITIONS banner in zz_generated_wire.go")
	}
	got := f.Tag.Get("json")
	if got != "dexConfigSecretRef,omitempty" {
		t.Fatalf("KargoOidcConfig.DexConfigSecretRef json tag: got %q, want %q", got, "dexConfigSecretRef,omitempty")
	}
}

// TestWire_ManualAdditionsBanner confirms the zz_generated_wire.go
// file carries the MANUAL ADDITIONS banner so anyone running the
// upstream sync sees the warning at the top of the file before
// clobbering it. Using a file read (not an embed) keeps the test's
// expectation aligned with what lives on disk today.
func TestWire_ManualAdditionsBanner(t *testing.T) {
	content := wireSource
	markers := []string{
		"MANUAL ADDITIONS",
		"KargoOidcConfig.DexConfigSecretRef",
		"Tagged with `// +optional`",
	}
	for _, m := range markers {
		if !strings.Contains(content, m) {
			t.Fatalf("zz_generated_wire.go missing marker %q — banner was overwritten by regen", m)
		}
	}
}

// TestWire_KargoOidcConfigFieldsOptional pins the `// +optional`
// markers on the KargoOidcConfig fields that controller-gen would
// otherwise emit as `required` (because upstream has no omitempty on
// them). Losing these markers silently regresses the generated CRD
// back to the admission-hostile shape where ref-only OIDC manifests
// are rejected because they omit zero-valued sibling fields.
//
// Scans the source rather than reflecting on the struct because
// `// +optional` lives in comments and disappears from the reflected
// type at runtime.
func TestWire_KargoOidcConfigFieldsOptional(t *testing.T) {
	content := wireSource
	// Cut to the KargoOidcConfig struct so matches elsewhere in the
	// file don't produce false positives.
	const marker = "type KargoOidcConfig struct {"
	start := strings.Index(content, marker)
	if start < 0 {
		t.Fatalf("KargoOidcConfig declaration missing from wire file")
	}
	body := content[start:]
	if end := strings.Index(body, "\n}\n"); end >= 0 {
		body = body[:end]
	}
	fields := []string{
		"Enabled",
		"DexEnabled",
		"DexConfig",
		"DexConfigSecret",
		"IssuerURL",
		"ClientID",
		"CliClientID",
		"AdminAccount",
		"ViewerAccount",
		"AdditionalScopes",
		"UserAccount",
		"ProjectCreatorAccount",
	}
	for _, f := range fields {
		// Anchor to a tab-indented field declaration terminated by a
		// space. This skips substring hits inside comment prose.
		needle := "\n\t" + f + " "
		idx := strings.Index(body, needle)
		if idx < 0 {
			t.Fatalf("KargoOidcConfig.%s missing — wire struct shape changed", f)
		}
		// Preceding comment line — should contain `+optional`.
		preceding := body[:idx]
		prevLineStart := strings.LastIndex(preceding, "\n")
		comment := strings.TrimSpace(preceding[prevLineStart+1:])
		if !strings.Contains(comment, "+optional") {
			t.Fatalf("KargoOidcConfig.%s: preceding line %q missing `// +optional` — CRD will regress to required", f, comment)
		}
	}
}

