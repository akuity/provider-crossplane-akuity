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
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"
)

type remarshalBadInput struct {
	Value float64
}

func TestRemarshalTo(t *testing.T) {
	type src struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	type dst struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	in := src{Name: "foo", Count: 3}
	var out dst
	if err := RemarshalTo(in, &out); err != nil {
		t.Fatalf("RemarshalTo: %v", err)
	}
	want := dst{Name: "foo", Count: 3}
	if diff := cmp.Diff(want, out); diff != "" {
		t.Fatalf("unexpected result (-want +got):\n%s", diff)
	}
}

func TestAPIModelToPBStruct(t *testing.T) {
	type model struct {
		A string         `json:"a"`
		B map[string]int `json:"b"`
	}

	got, err := APIModelToPBStruct(model{A: "x", B: map[string]int{"n": 1}})
	if err != nil {
		t.Fatalf("APIModelToPBStruct: %v", err)
	}

	want, err := structpb.NewStruct(map[string]any{
		"a": "x",
		"b": map[string]any{"n": float64(1)},
	})
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}
	if diff := cmp.Diff(want.AsMap(), got.AsMap()); diff != "" {
		t.Fatalf("unexpected result (-want +got):\n%s", diff)
	}
}

func TestRemarshalTo_MarshalError(t *testing.T) {
	var target remarshalBadInput
	if err := RemarshalTo(remarshalBadInput{Value: math.NaN()}, &target); err == nil {
		t.Fatal("expected error from NaN input, got nil")
	}
}

func TestRemarshalTo_UnmarshalError(t *testing.T) {
	target := make(map[string]string)
	if err := RemarshalTo(remarshalBadInput{Value: 12}, &target); err == nil {
		t.Fatal("expected error when decoding struct into map[string]string, got nil")
	}
}

func TestAPIModelToPBStruct_MarshalError(t *testing.T) {
	got, err := APIModelToPBStruct(remarshalBadInput{Value: math.NaN()})
	if err == nil {
		t.Fatal("expected error from NaN input, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil Struct on error, got %v", got)
	}
}

func TestProtoToWire_RoundTrip(t *testing.T) {
	type wire struct {
		A string         `json:"a"`
		B map[string]any `json:"b"`
	}
	src, err := structpb.NewStruct(map[string]any{
		"a": "x",
		"b": map[string]any{"n": float64(1)},
	})
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}

	got, err := ProtoToWire[wire](src)
	if err != nil {
		t.Fatalf("ProtoToWire: %v", err)
	}
	want := wire{A: "x", B: map[string]any{"n": float64(1)}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unexpected result (-want +got):\n%s", diff)
	}
}

func TestProtoToWire_NilInputReturnsZero(t *testing.T) {
	type wire struct {
		A string `json:"a"`
	}
	got, err := ProtoToWire[wire](nil)
	if err != nil {
		t.Fatalf("ProtoToWire: %v", err)
	}
	if diff := cmp.Diff(wire{}, got); diff != "" {
		t.Fatalf("expected zero value, got diff:\n%s", diff)
	}
}

func TestProtoToWire_TypedNilReturnsZero(t *testing.T) {
	type wire struct {
		A string `json:"a"`
	}
	var src *structpb.Struct
	got, err := ProtoToWire[wire](src)
	if err != nil {
		t.Fatalf("ProtoToWire: %v", err)
	}
	if diff := cmp.Diff(wire{}, got); diff != "" {
		t.Fatalf("expected zero value, got diff:\n%s", diff)
	}
}
