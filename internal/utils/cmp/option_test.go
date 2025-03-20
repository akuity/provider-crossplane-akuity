package cmp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

type TestStruct struct {
	Name       string
	Age        int
	IsActive   bool
	Tags       []string
	Properties map[string]string
	Child      *ChildStruct
}

type ChildStruct struct {
	ID    int
	Info  string
	Slice []string
}

func TestEquateEmpty(t *testing.T) {
	tests := []struct {
		name     string
		x        any
		y        any
		expected bool
	}{
		{
			name: "identical non-empty structs",
			x: TestStruct{
				Name:       "test",
				Age:        30,
				IsActive:   true,
				Tags:       []string{"tag1", "tag2"},
				Properties: map[string]string{"key": "value"},
				Child: &ChildStruct{
					ID:    1,
					Info:  "info",
					Slice: []string{"a", "b"},
				},
			},
			y: TestStruct{
				Name:       "test",
				Age:        30,
				IsActive:   true,
				Tags:       []string{"tag1", "tag2"},
				Properties: map[string]string{"key": "value"},
				Child: &ChildStruct{
					ID:    1,
					Info:  "info",
					Slice: []string{"a", "b"},
				},
			},
			expected: true,
		},
		{
			name: "nil slice vs empty slice",
			x: TestStruct{
				Name: "test",
				Tags: nil,
			},
			y: TestStruct{
				Name: "test",
				Tags: []string{},
			},
			expected: true,
		},
		{
			name: "nil map vs empty map",
			x: TestStruct{
				Name:       "test",
				Properties: nil,
			},
			y: TestStruct{
				Name:       "test",
				Properties: map[string]string{},
			},
			expected: true,
		},
		{
			name: "nil pointer vs empty struct pointer",
			x: TestStruct{
				Name:  "test",
				Child: nil,
			},
			y: TestStruct{
				Name: "test",
				Child: &ChildStruct{
					Slice: []string{},
				},
			},
			expected: true,
		},
		{
			name: "different non-empty values",
			x: TestStruct{
				Name: "test1",
				Tags: []string{"tag1"},
			},
			y: TestStruct{
				Name: "test2",
				Tags: []string{"tag2"},
			},
			expected: false,
		},
		{
			name: "empty vs non-empty",
			x:    TestStruct{},
			y: TestStruct{
				Name: "test",
			},
			expected: false,
		},
		{
			name: "nested empty structs",
			x: TestStruct{
				Name:  "test",
				Child: &ChildStruct{},
			},
			y: TestStruct{
				Name:  "test",
				Child: nil,
			},
			expected: true,
		},
		{
			name:     "completely empty structs",
			x:        TestStruct{},
			y:        TestStruct{},
			expected: true,
		},
		{
			name:     "nil vs empty struct",
			x:        nil,
			y:        TestStruct{},
			expected: false, // Different types, should not be equal
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := EquateEmpty()
			result := cmp.Equal(tt.x, tt.y, opts...)
			if result != tt.expected {
				t.Errorf("cmp.Equal(%v, %v) = %v, expected %v", tt.x, tt.y, result, tt.expected)
				if !result {
					diff := cmp.Diff(tt.x, tt.y, opts...)
					t.Logf("Diff: %s", diff)
				}
			}
		})
	}
}
