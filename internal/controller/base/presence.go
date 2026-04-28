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

package base

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FieldPresence records the JSON fields a user supplied under
// spec.forProvider. Object fields have Children populated; scalar and
// list fields are treated as whole-field ownership.
type FieldPresence struct {
	Object   bool
	Present  bool
	Children map[string]FieldPresence
}

// ForProviderPresence reads the live managed resource as unstructured
// data and returns the user-supplied spec.forProvider field tree. The
// typed managed object cannot preserve "omitted" vs "explicit zero"
// for omitempty scalars, so controllers should prefer this raw read
// when they need user-intent drift.
//
// If no kube reader is available (mostly unit tests), the function
// falls back to the typed object. If the raw read fails, it returns nil
// so callers keep the pre-existing full-struct comparison instead of
// making a degraded presence-based decision.
func ForProviderPresence(ctx context.Context, kube client.Reader, obj client.Object, gvk schema.GroupVersionKind) *FieldPresence {
	if kube != nil && !gvk.Empty() {
		raw := &unstructured.Unstructured{}
		raw.SetGroupVersionKind(gvk)
		if err := kube.Get(ctx, client.ObjectKeyFromObject(obj), raw); err == nil {
			return forProviderPresenceFromUnstructured(raw.Object)
		}
		return nil
	}

	return ForProviderPresenceFromObject(obj)
}

// ForProviderPresenceFromObject derives presence from a typed object.
// This is a best-effort fallback: JSON omitempty means explicit zero
// scalar values are not distinguishable from absent fields.
func ForProviderPresenceFromObject(obj client.Object) *FieldPresence {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil
	}
	return forProviderPresenceFromUnstructured(raw)
}

func forProviderPresenceFromUnstructured(raw map[string]interface{}) *FieldPresence {
	fp, ok, _ := unstructured.NestedMap(raw, "spec", "forProvider")
	if !ok {
		return nil
	}
	p := presenceFromMap(fp)
	return &p
}

func presenceFromMap(m map[string]interface{}) FieldPresence {
	p := FieldPresence{Object: true, Present: true}
	if len(m) == 0 {
		return p
	}
	p.Children = make(map[string]FieldPresence, len(m))
	for k, v := range m {
		p.Children[k] = presenceFromValue(v)
	}
	return p
}

func presenceFromValue(v interface{}) FieldPresence {
	if m, ok := v.(map[string]interface{}); ok {
		return presenceFromMap(m)
	}
	return FieldPresence{Present: true}
}

// ProjectByPresence returns a copy of in containing only the fields
// described by presence. It uses reflection over JSON tags instead of
// JSON round-tripping so explicit zero values such as false, 0, and
// "" remain comparable when the raw object proves the user supplied
// the field.
func ProjectByPresence[T any](in *T, presence FieldPresence) T {
	var zero T
	if in == nil || !presence.Present {
		return zero
	}
	projected := projectValue(reflect.ValueOf(*in), presence)
	if !projected.IsValid() {
		return zero
	}
	if out, ok := projected.Interface().(T); ok {
		return out
	}
	return zero
}

func projectValue(src reflect.Value, presence FieldPresence) reflect.Value { //nolint:gocyclo
	if !src.IsValid() {
		return src
	}
	for src.Kind() == reflect.Interface {
		if src.IsNil() {
			return reflect.Zero(src.Type())
		}
		src = src.Elem()
	}

	if !presence.Present {
		return reflect.Zero(src.Type())
	}
	if !presence.Object {
		return src
	}

	switch src.Kind() { //nolint:exhaustive // only Pointer/Struct/Map need recursive projection; other kinds fall through to whole-field copy.
	case reflect.Pointer:
		if src.IsNil() {
			return reflect.Zero(src.Type())
		}
		projected := projectValue(src.Elem(), presence)
		if !projected.IsValid() {
			return reflect.Zero(src.Type())
		}
		out := reflect.New(src.Type().Elem())
		out.Elem().Set(projected)
		return out
	case reflect.Struct:
		return projectStruct(src, presence)
	case reflect.Map:
		return projectMap(src, presence)
	default:
		return src
	}
}

func projectStruct(src reflect.Value, presence FieldPresence) reflect.Value {
	dst := reflect.New(src.Type()).Elem()
	if len(presence.Children) == 0 {
		return dst
	}
	fields := jsonFields(src.Type())
	for name, child := range presence.Children {
		idx, ok := fields[name]
		if !ok {
			continue
		}
		sf, ok := fieldByIndex(src, idx, false)
		if !ok {
			continue
		}
		df, ok := fieldByIndex(dst, idx, true)
		if !ok {
			continue
		}
		if !df.CanSet() {
			continue
		}
		projected := projectValue(sf, child)
		if projected.IsValid() && projected.Type().AssignableTo(df.Type()) {
			df.Set(projected)
		}
	}
	return dst
}

func projectMap(src reflect.Value, presence FieldPresence) reflect.Value {
	if src.IsNil() || src.Type().Key().Kind() != reflect.String {
		// Presence paths come from JSON object keys, so non-string maps
		// cannot be described safely by this projection.
		return reflect.Zero(src.Type())
	}
	dst := reflect.MakeMapWithSize(src.Type(), len(presence.Children))
	for name, child := range presence.Children {
		key := reflect.ValueOf(name).Convert(src.Type().Key())
		value := src.MapIndex(key)
		if !value.IsValid() {
			continue
		}
		projected := projectValue(value, child)
		if projected.IsValid() && projected.Type().AssignableTo(src.Type().Elem()) {
			dst.SetMapIndex(key, projected)
		}
	}
	return dst
}

func fieldByIndex(v reflect.Value, index []int, allocate bool) (reflect.Value, bool) {
	for _, i := range index {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !allocate || !v.CanSet() {
					return reflect.Value{}, false
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v, true
}

func jsonFields(t reflect.Type) map[string][]int {
	fields := make(map[string][]int, t.NumField())
	collectJSONFields(t, nil, fields)
	return fields
}

func collectJSONFields(t reflect.Type, prefix []int, fields map[string][]int) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		tag := f.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}

		index := append(append([]int(nil), prefix...), i)
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if f.Anonymous && name == "" && ft.Kind() == reflect.Struct {
			collectJSONFields(ft, index, fields)
			continue
		}
		if name == "" {
			name = f.Name
		}
		fields[name] = index
	}
}
