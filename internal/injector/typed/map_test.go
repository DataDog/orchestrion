// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapType_Matches(t *testing.T) {
	tests := []struct {
		name     string
		mapType  *MapType
		node     dst.Expr
		expected bool
	}{
		{
			name:     "matches map[string]int",
			mapType:  &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}},
			node:     &dst.MapType{Key: dst.NewIdent("string"), Value: dst.NewIdent("int")},
			expected: true,
		},
		{
			name:     "does not match non-map",
			mapType:  &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}},
			node:     dst.NewIdent("string"),
			expected: false,
		},
		{
			name:     "does not match map with different key type",
			mapType:  &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}},
			node:     &dst.MapType{Key: dst.NewIdent("int"), Value: dst.NewIdent("int")},
			expected: false,
		},
		{
			name:     "does not match map with different value type",
			mapType:  &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}},
			node:     &dst.MapType{Key: dst.NewIdent("string"), Value: dst.NewIdent("string")},
			expected: false,
		},
		{
			name:     "matches map with qualified types",
			mapType:  &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{ImportPath: "fmt", Name: "Stringer"}},
			node:     &dst.MapType{Key: dst.NewIdent("string"), Value: &dst.SelectorExpr{X: dst.NewIdent("fmt"), Sel: dst.NewIdent("Stringer")}},
			expected: true,
		},
		{
			name:     "matches map with pointer value",
			mapType:  &MapType{Key: &NamedType{Name: "string"}, Value: &PointerType{Elem: &NamedType{Name: "User"}}},
			node:     &dst.MapType{Key: dst.NewIdent("string"), Value: &dst.StarExpr{X: dst.NewIdent("User")}},
			expected: true,
		},
		{
			name:     "matches nested map",
			mapType:  &MapType{Key: &NamedType{Name: "string"}, Value: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}}},
			node:     &dst.MapType{Key: dst.NewIdent("string"), Value: &dst.MapType{Key: dst.NewIdent("string"), Value: dst.NewIdent("int")}},
			expected: true,
		},
		{
			name:     "matches map with slice value",
			mapType:  &MapType{Key: &NamedType{Name: "string"}, Value: &SliceType{Elem: &NamedType{Name: "int"}}},
			node:     &dst.MapType{Key: dst.NewIdent("string"), Value: &dst.ArrayType{Elt: dst.NewIdent("int")}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mapType.Matches(tt.node)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapType_AsNode(t *testing.T) {
	tests := []struct {
		name     string
		mapType  *MapType
		expected dst.Expr
	}{
		{
			name:    "map of simple types",
			mapType: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}},
			expected: &dst.MapType{
				Key:   &dst.Ident{Name: "string"},
				Value: &dst.Ident{Name: "int"},
			},
		},
		{
			name:    "map with qualified value type",
			mapType: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{ImportPath: "fmt", Name: "Stringer"}},
			expected: &dst.MapType{
				Key:   &dst.Ident{Name: "string"},
				Value: &dst.Ident{Name: "Stringer", Path: "fmt"},
			},
		},
		{
			name:    "map with pointer value",
			mapType: &MapType{Key: &NamedType{Name: "string"}, Value: &PointerType{Elem: &NamedType{Name: "User"}}},
			expected: &dst.MapType{
				Key:   &dst.Ident{Name: "string"},
				Value: &dst.StarExpr{X: &dst.Ident{Name: "User"}},
			},
		},
		{
			name:    "nested map",
			mapType: &MapType{Key: &NamedType{Name: "string"}, Value: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}}},
			expected: &dst.MapType{
				Key: &dst.Ident{Name: "string"},
				Value: &dst.MapType{
					Key:   &dst.Ident{Name: "string"},
					Value: &dst.Ident{Name: "int"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mapType.AsNode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewType_MapParsing(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    Type
		expectError bool
	}{
		{
			name:     "simple map",
			input:    "map[string]int",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}},
		},
		{
			name:     "map with qualified value",
			input:    "map[string]net/http.Request",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{ImportPath: "net/http", Name: "Request"}},
		},
		{
			name:     "map with pointer value",
			input:    "map[string]*User",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &PointerType{Elem: &NamedType{Name: "User"}}},
		},
		{
			name:     "map with qualified pointer value",
			input:    "map[string]*net/http.Request",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &PointerType{Elem: &NamedType{ImportPath: "net/http", Name: "Request"}}},
		},
		{
			name:     "nested map",
			input:    "map[string]map[string]int",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &MapType{Key: &NamedType{Name: "string"}, Value: &NamedType{Name: "int"}}},
		},
		{
			name:     "map with slice value",
			input:    "map[string][]int",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &SliceType{Elem: &NamedType{Name: "int"}}},
		},
		{
			name:     "complex nested map",
			input:    "map[string]map[int][]*User",
			expected: &MapType{Key: &NamedType{Name: "string"}, Value: &MapType{Key: &NamedType{Name: "int"}, Value: &SliceType{Elem: &PointerType{Elem: &NamedType{Name: "User"}}}}},
		},
		{
			name:        "map with invalid syntax",
			input:       "map[string",
			expectError: true,
		},
		{
			name:        "map without key type",
			input:       "map[]int",
			expectError: true,
		},
		{
			name:        "map without value type",
			input:       "map[string]",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewType(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
