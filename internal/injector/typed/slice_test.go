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

func TestSliceType_Matches(t *testing.T) {
	tests := []struct {
		name     string
		slice    *SliceType
		node     dst.Expr
		expected bool
	}{
		{
			name:     "matches slice of string",
			slice:    &SliceType{Elem: &NamedType{Name: "string"}},
			node:     &dst.ArrayType{Elt: dst.NewIdent("string")},
			expected: true,
		},
		{
			name:     "does not match non-slice",
			slice:    &SliceType{Elem: &NamedType{Name: "string"}},
			node:     dst.NewIdent("string"),
			expected: false,
		},
		{
			name:     "does not match array (has length)",
			slice:    &SliceType{Elem: &NamedType{Name: "string"}},
			node:     &dst.ArrayType{Len: &dst.BasicLit{Value: "10"}, Elt: dst.NewIdent("string")},
			expected: false,
		},
		{
			name:     "does not match slice of different type",
			slice:    &SliceType{Elem: &NamedType{Name: "string"}},
			node:     &dst.ArrayType{Elt: dst.NewIdent("int")},
			expected: false,
		},
		{
			name:     "matches slice of qualified type",
			slice:    &SliceType{Elem: &NamedType{ImportPath: "fmt", Name: "Stringer"}},
			node:     &dst.ArrayType{Elt: &dst.SelectorExpr{X: dst.NewIdent("fmt"), Sel: dst.NewIdent("Stringer")}},
			expected: true,
		},
		{
			name:     "matches slice of pointer type",
			slice:    &SliceType{Elem: &PointerType{Elem: &NamedType{Name: "string"}}},
			node:     &dst.ArrayType{Elt: &dst.StarExpr{X: dst.NewIdent("string")}},
			expected: true,
		},
		{
			name:     "matches nested slice",
			slice:    &SliceType{Elem: &SliceType{Elem: &NamedType{Name: "string"}}},
			node:     &dst.ArrayType{Elt: &dst.ArrayType{Elt: dst.NewIdent("string")}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.slice.Matches(tt.node)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSliceType_AsNode(t *testing.T) {
	tests := []struct {
		name     string
		slice    *SliceType
		expected dst.Expr
	}{
		{
			name:  "slice of simple type",
			slice: &SliceType{Elem: &NamedType{Name: "string"}},
			expected: &dst.ArrayType{
				Elt: &dst.Ident{Name: "string"},
			},
		},
		{
			name:  "slice of qualified type",
			slice: &SliceType{Elem: &NamedType{ImportPath: "fmt", Name: "Stringer"}},
			expected: &dst.ArrayType{
				Elt: &dst.Ident{Name: "Stringer", Path: "fmt"},
			},
		},
		{
			name:  "slice of pointer",
			slice: &SliceType{Elem: &PointerType{Elem: &NamedType{Name: "string"}}},
			expected: &dst.ArrayType{
				Elt: &dst.StarExpr{X: &dst.Ident{Name: "string"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.slice.AsNode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewType_SliceParsing(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    Type
		expectError bool
	}{
		{
			name:     "simple slice",
			input:    "[]string",
			expected: &SliceType{Elem: &NamedType{Name: "string"}},
		},
		{
			name:     "slice of qualified type",
			input:    "[]net/http.Request",
			expected: &SliceType{Elem: &NamedType{ImportPath: "net/http", Name: "Request"}},
		},
		{
			name:     "slice of pointer",
			input:    "[]*string",
			expected: &SliceType{Elem: &PointerType{Elem: &NamedType{Name: "string"}}},
		},
		{
			name:     "slice of pointer to qualified type",
			input:    "[]*net/http.Request",
			expected: &SliceType{Elem: &PointerType{Elem: &NamedType{ImportPath: "net/http", Name: "Request"}}},
		},
		{
			name:     "nested slice",
			input:    "[][]string",
			expected: &SliceType{Elem: &SliceType{Elem: &NamedType{Name: "string"}}},
		},
		{
			name:     "complex nested slice",
			input:    "[][]*net/http.Request",
			expected: &SliceType{Elem: &SliceType{Elem: &PointerType{Elem: &NamedType{ImportPath: "net/http", Name: "Request"}}}},
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
