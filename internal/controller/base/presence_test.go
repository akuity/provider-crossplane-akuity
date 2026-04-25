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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type presenceChild struct {
	Name    string `json:"name,omitempty"`
	Ignored string `json:"ignored,omitempty"`
}

type presenceSample struct {
	Name    string            `json:"name,omitempty"`
	Flag    bool              `json:"flag,omitempty"`
	Count   int               `json:"count,omitempty"`
	Child   *presenceChild    `json:"child,omitempty"`
	Data    map[string]string `json:"data,omitempty"`
	Items   []string          `json:"items,omitempty"`
	Ignored string            `json:"ignored,omitempty"`
}

type presenceOddSample struct {
	IntMap map[int]string `json:"intMap,omitempty"`
}

type presenceEmbeddedFields struct {
	Promoted string `json:"promoted,omitempty"`
	Hidden   string `json:"hidden,omitempty"`
}

type presenceEmbeddedSample struct {
	presenceEmbeddedFields
	Name string `json:"name,omitempty"`
}

type failingReader struct{}

func (failingReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return errors.New("boom")
}

func (failingReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errors.New("boom")
}

func TestForProviderPresence_RawReadFailureFailsClosed(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetName("sample")
	obj.SetNamespace("ns")

	got := ForProviderPresence(
		context.Background(),
		failingReader{},
		obj,
		schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "Sample"},
	)

	assert.Nil(t, got, "raw read failure must keep full-struct comparison")
}

func TestProjectByPresence_PreservesExplicitZeroValues(t *testing.T) {
	presence := presenceFromMap(map[string]interface{}{
		"flag": false,
		"name": "",
	})
	in := presenceSample{
		Name:    "",
		Flag:    false,
		Count:   42,
		Ignored: "server-default",
	}

	got := ProjectByPresence(&in, presence)

	assert.Equal(t, presenceSample{Name: "", Flag: false}, got)
}

func TestProjectByPresence_NilPointerFieldDoesNotPanic(t *testing.T) {
	presence := presenceFromMap(map[string]interface{}{
		"child": map[string]interface{}{"name": "nested"},
	})
	in := presenceSample{}

	got := ProjectByPresence(&in, presence)

	assert.Nil(t, got.Child)
}

func TestProjectByPresence_NonStringMapKeyIsIgnored(t *testing.T) {
	presence := presenceFromMap(map[string]interface{}{
		"intMap": map[string]interface{}{"1": "one"},
	})
	in := presenceOddSample{IntMap: map[int]string{1: "one"}}

	got := ProjectByPresence(&in, presence)

	assert.Nil(t, got.IntMap)
}

func TestProjectByPresence_SliceIsWholeField(t *testing.T) {
	presence := presenceFromMap(map[string]interface{}{
		"items": []interface{}{"one"},
	})
	in := presenceSample{
		Items:   []string{"one", "two"},
		Ignored: "server-default",
	}

	got := ProjectByPresence(&in, presence)

	assert.Equal(t, []string{"one", "two"}, got.Items)
	assert.Empty(t, got.Ignored)
}

func TestProjectByPresence_EmbeddedJSONFields(t *testing.T) {
	presence := presenceFromMap(map[string]interface{}{
		"promoted": "value",
	})
	in := presenceEmbeddedSample{
		presenceEmbeddedFields: presenceEmbeddedFields{
			Promoted: "keep",
			Hidden:   "drop",
		},
		Name: "drop",
	}

	got := ProjectByPresence(&in, presence)

	assert.Equal(t, "keep", got.Promoted)
	assert.Empty(t, got.Hidden)
	assert.Empty(t, got.Name)
}

func TestDriftSpec_PresenceIgnoresUnspecifiedServerDefaults(t *testing.T) {
	presence := presenceFromMap(map[string]interface{}{
		"name": "user",
		"child": map[string]interface{}{
			"name": "nested",
		},
		"data": map[string]interface{}{
			"owned": "v",
		},
	})
	desired := presenceSample{
		Name:  "same",
		Count: 1,
		Child: &presenceChild{Name: "nested", Ignored: "desired"},
		Data:  map[string]string{"owned": "v", "unowned": "desired"},
		Items: []string{"desired"},
	}
	observed := presenceSample{
		Name:  "same",
		Count: 99,
		Child: &presenceChild{Name: "nested", Ignored: "observed"},
		Data:  map[string]string{"owned": "v", "unowned": "observed"},
		Items: []string{"observed"},
	}

	ok, err := DriftSpec[presenceSample]{Presence: &presence}.UpToDate(context.Background(), &desired, &observed)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestDriftSpec_PresenceDetectsOwnedMapKeyDrift(t *testing.T) {
	presence := presenceFromMap(map[string]interface{}{
		"data": map[string]interface{}{
			"owned": "v",
		},
	})
	desired := presenceSample{Data: map[string]string{"owned": "desired"}}
	observed := presenceSample{Data: map[string]string{"owned": "observed", "unowned": "server"}}

	ok, err := DriftSpec[presenceSample]{Presence: &presence}.UpToDate(context.Background(), &desired, &observed)
	require.NoError(t, err)
	assert.False(t, ok)
}
