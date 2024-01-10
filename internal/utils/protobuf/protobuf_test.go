package protobuf_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/akuityio/provider-crossplane-akuity/internal/utils/protobuf"
)

type RemarshalError struct {
	Value float64
}

func TestRemarshalObject(t *testing.T) {
	obj := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	target := make(map[string]interface{})

	err := protobuf.RemarshalObject(obj, &target)

	require.NoError(t, err)
	assert.Equal(t, obj, target)

	// Check that the target object is a different instance
	assert.NotSame(t, &obj, &target)

	// Check that the target object is a deep copy of the original object
	obj["key1"] = "modified"
	assert.NotEqual(t, obj, target)

	// Check that the target object can be marshaled and unmarshaled again without error
	data, err := json.Marshal(target)
	require.NoError(t, err)

	newTarget := make(map[string]interface{})
	err = json.Unmarshal(data, &newTarget)
	require.NoError(t, err)
	assert.Equal(t, target, newTarget)
}

func TestRemarshalObject_Error(t *testing.T) {
	target := RemarshalError{}
	err := protobuf.RemarshalObject(RemarshalError{Value: math.NaN()}, &target)
	require.Error(t, err)

	newTarget := make(map[string]string)
	err = protobuf.RemarshalObject(RemarshalError{Value: float64(12)}, &newTarget)
	require.Error(t, err)
}

func TestMarshalObjectToProtobufStruct(t *testing.T) {
	obj := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	expectedStruct := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"key1": {
				Kind: &structpb.Value_StringValue{
					StringValue: "value1",
				},
			},
			"key2": {
				Kind: &structpb.Value_StringValue{
					StringValue: "value2",
				},
			},
		},
	}

	result, err := protobuf.MarshalObjectToProtobufStruct(obj)
	require.NoError(t, err)
	require.Equal(t, expectedStruct, result)
}

func TestMarshalObjectToProtobufStruct_Error(t *testing.T) {
	result, err := protobuf.MarshalObjectToProtobufStruct(RemarshalError{Value: math.NaN()})
	require.Error(t, err)
	assert.Nil(t, result)
}
