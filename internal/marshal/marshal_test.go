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
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"
)

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
