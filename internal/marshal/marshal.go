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

// Package marshal provides JSON, map, and protobuf helpers used by the
// provider's convert layer. The helpers preserve protojson camelCase
// keys and omit unpopulated fields so gateway payloads round-trip with
// the same shape as the source wire types.
package marshal

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// RemarshalTo converts obj into target by serializing obj to JSON and
// deserializing it into target. Useful for shape-compatible types that
// share JSON tags.
func RemarshalTo(obj, target any) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return nil
}

// APIModelToPBStruct converts a Go API model into a structpb.Struct via
// JSON round-trip. Callers use this to embed an arbitrary Go object in a
// protobuf request field typed as google.protobuf.Struct.
func APIModelToPBStruct(obj any) (*structpb.Struct, error) {
	m := map[string]any{}
	if err := RemarshalTo(obj, &m); err != nil {
		return nil, err
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, fmt.Errorf("new struct: %w", err)
	}
	return s, nil
}

// ProtoToMap marshals a protobuf message to JSON (protojson, without
// unpopulated fields) and decodes it into a map[string]any. Keys follow
// protojson camelCase conventions.
func ProtoToMap(msg proto.Message) (map[string]any, error) {
	data, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("protojson marshal: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return m, nil
}

// GoModelToProto populates the supplied protobuf message from a Go
// model via JSON to protojson. Used by the Kargo controllers to bridge
// hand-authored akuity wire types (JSON-tagged) into the protobuf
// request types the gateway client expects.
func GoModelToProto(obj any, target proto.Message) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	opts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := opts.Unmarshal(data, target); err != nil {
		return fmt.Errorf("protojson unmarshal: %w", err)
	}
	return nil
}

// ProtoToWire decodes a protobuf message into the hand-authored Akuity
// Go-wire struct T. It is the inverse of GoModelToProto and composes
// ProtoToMap + RemarshalTo so every controller converter can drop its
// own two-step bridge helper. Nil / zero-valued input (including a
// typed-nil pointer wrapped in the proto.Message interface) yields the
// zero value of T so callers can propagate an absent sub-message
// without a special case.
func ProtoToWire[T any](msg proto.Message) (T, error) {
	var zero T
	if isNilProto(msg) {
		return zero, nil
	}
	m, err := ProtoToMap(msg)
	if err != nil {
		return zero, err
	}
	if err := RemarshalTo(m, &zero); err != nil {
		return zero, err
	}
	return zero, nil
}

func isNilProto(msg proto.Message) bool {
	if msg == nil {
		return true
	}
	r := msg.ProtoReflect()
	return r == nil || !r.IsValid()
}
