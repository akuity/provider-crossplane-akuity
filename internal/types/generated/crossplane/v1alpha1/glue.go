/*
Copyright 2026 Akuity, Inc.
Licensed under the Apache License, Version 2.0.
*/

// glue.go hosts hand-written adapter helpers referenced by the
// generated converter functions (<section>_convert.go). Each adapter
// handles a shape mismatch between the curated Crossplane-provider
// types and the upstream Akuity wire types; the rest of the convert
// layer is mechanical field copying emitted from gencrossplaneconverts.

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
)

// ValidateKustomizationYAML returns an error when s is non-empty and
// cannot be parsed as YAML, OR parses to a non-object top level.
// Controllers call this at Create/Update entry to reject malformed
// payloads before the generated converter silently drops them. Object
// shape is required because the downstream bridge feeds a
// runtime.RawExtension / protobuf Struct that only accepts a mapping
// top-level; a scalar or sequence is syntactically valid YAML but
// produces a semantically broken kustomization document.
func ValidateKustomizationYAML(s string) error {
	if s == "" {
		return nil
	}
	raw, err := yaml.YAMLToJSON([]byte(s))
	if err != nil {
		return fmt.Errorf("invalid kustomization YAML: %w", err)
	}
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return fmt.Errorf("kustomization YAML must be an object at the top level: %w", err)
	}
	return nil
}

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
// for display in a managed resource's observed state. An empty
// extension yields an empty string. JSON→YAML conversion failures are
// treated the same way (empty string) rather than propagated because
// the converters run in observe paths where a partial status is
// preferable to bubbling up an error.
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

// StringPtrToTimePtr parses an RFC3339 string pointer into a
// *metav1.Time. The curated/v1alpha1 Crossplane shape rewrites
// metav1.Time fields to string for CRD friendliness; the Akuity wire
// type retains metav1.Time, and this adapter bridges the two on the
// outbound path. Nil or empty input yields nil so the converter can
// pass the field through transparently.
func StringPtrToTimePtr(s *string) *metav1.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	return &metav1.Time{Time: t}
}

// TimePtrToStringPtr renders a *metav1.Time back to an RFC3339
// *string for the curated observed state. Nil or zero time yields
// nil.
func TimePtrToStringPtr(t *metav1.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// DexConfigSecretResolvedToAPI wraps a flat map of dex configuration
// secret values into the map[string]akuitytypes.Value shape the Akuity
// gateway expects. Each entry becomes {value: "..."}; nil/empty input
// yields nil so callers can compose without pre-checks. Exposed so the
// KargoInstance controller can inject resolved Secret data into the
// wire payload after the generated converter has run (the curated
// surface stores only a LocalObjectReference, not the wire-shape map).
func DexConfigSecretResolvedToAPI(resolved map[string]string) map[string]akuitytypes.Value {
	if len(resolved) == 0 {
		return nil
	}
	out := make(map[string]akuitytypes.Value, len(resolved))
	for k, v := range resolved {
		v := v
		out[k] = akuitytypes.Value{Value: &v}
	}
	return out
}
