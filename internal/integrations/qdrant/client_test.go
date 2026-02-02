// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-02-02
// Last Modified: 2026-02-02

package qdrant

import (
	"testing"

	pb "github.com/qdrant/go-client/qdrant"
)

func TestValueConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"string", "hello", "hello"},
		{"int", 123, int64(123)}, // Qdrant stores ints as int64
		{"float", 3.14, 3.14},
		{"bool", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to Qdrant value
			qVal := toQdrantValue(tt.input)

			// Convert back to Go value
			goVal := fromQdrantValue(qVal)

			if goVal != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, goVal)
			}
		})
	}
}

func TestNilValue(t *testing.T) {
	qVal := &pb.Value{Kind: &pb.Value_NullValue{}}
	goVal := fromQdrantValue(qVal)
	if goVal != nil {
		t.Errorf("Expected nil for null value, got %v", goVal)
	}
}
