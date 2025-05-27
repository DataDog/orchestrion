// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArrayType_Matches(t *testing.T) {
	tests := []struct {
		name     string
		array    *ArrayType
		node     dst.Expr
		expected bool
	}{
		{
			name:     "matches array of string with same size",
			array:    &ArrayType{Size: 10, Elem: &NamedType{Name: "string"}},
			node:     &dst.ArrayType{Len: &dst.BasicLit{Value: "10"}, Elt: dst.NewIdent("string")},
			expected: true,
		},
		{
			name:     "does not match array with different size",
			array:    &ArrayType{Size: 10, Elem: &NamedType{Name: "string"}},
			node:     &dst.ArrayType{Len: &dst.BasicLit{Value: "5"}, Elt: dst.NewIdent("string")},
			expected: false,
		},
		{
			name:     "does not match slice (no length)",
			array:    &ArrayType{Size: 10, Elem: &NamedType{Name: "string"}},
			node:     &dst.ArrayType{Elt: dst.NewIdent("string")},
			expected: false,
		},
		{
			name:     "does not match non-array",
			array:    &ArrayType{Size: 10, Elem: &NamedType{Name: "string"}},
			node:     dst.NewIdent("string"),
			expected: false,
		},
		{
			name:     "does not match array of different type",
			array:    &ArrayType{Size: 10, Elem: &NamedType{Name: "string"}},
			node:     &dst.ArrayType{Len: &dst.BasicLit{Value: "10"}, Elt: dst.NewIdent("int")},
			expected: false,
		},
		{
			name:     "matches array of qualified type",
			array:    &ArrayType{Size: 5, Elem: &NamedType{ImportPath: "fmt", Name: "Stringer"}},
			node:     &dst.ArrayType{Len: &dst.BasicLit{Value: "5"}, Elt: &dst.SelectorExpr{X: dst.NewIdent("fmt"), Sel: dst.NewIdent("Stringer")}},
			expected: true,
		},
		{
			name:     "matches array with hex size",
			array:    &ArrayType{Size: 16, Elem: &NamedType{Name: "byte"}},
			node:     &dst.ArrayType{Len: &dst.BasicLit{Value: "0x10"}, Elt: dst.NewIdent("byte")},
			expected: true,
		},
		{
			name:     "matches array with octal size",
			array:    &ArrayType{Size: 8, Elem: &NamedType{Name: "byte"}},
			node:     &dst.ArrayType{Len: &dst.BasicLit{Value: "010"}, Elt: dst.NewIdent("byte")},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.array.Matches(tt.node)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArrayType_AsNode(t *testing.T) {
	tests := []struct {
		name     string
		array    *ArrayType
		expected dst.Expr
	}{
		{
			name:  "array of simple type",
			array: &ArrayType{Size: 10, Elem: &NamedType{Name: "string"}},
			expected: &dst.ArrayType{
				Len: &dst.BasicLit{Kind: token.INT, Value: "10"},
				Elt: &dst.Ident{Name: "string"},
			},
		},
		{
			name:  "array of qualified type",
			array: &ArrayType{Size: 5, Elem: &NamedType{ImportPath: "fmt", Name: "Stringer"}},
			expected: &dst.ArrayType{
				Len: &dst.BasicLit{Kind: token.INT, Value: "5"},
				Elt: &dst.Ident{Name: "Stringer", Path: "fmt"},
			},
		},
		{
			name:  "array of pointer",
			array: &ArrayType{Size: 3, Elem: &PointerType{Elem: &NamedType{Name: "string"}}},
			expected: &dst.ArrayType{
				Len: &dst.BasicLit{Kind: token.INT, Value: "3"},
				Elt: &dst.StarExpr{X: &dst.Ident{Name: "string"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.array.AsNode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewType_ArrayParsing(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    Type
		expectError bool
	}{
		{
			name:     "simple array",
			input:    "[10]string",
			expected: &ArrayType{Size: 10, Elem: &NamedType{Name: "string"}},
		},
		{
			name:     "array of qualified type",
			input:    "[5]net/http.Request",
			expected: &ArrayType{Size: 5, Elem: &NamedType{ImportPath: "net/http", Name: "Request"}},
		},
		{
			name:     "array of pointer",
			input:    "[3]*string",
			expected: &ArrayType{Size: 3, Elem: &PointerType{Elem: &NamedType{Name: "string"}}},
		},
		{
			name:     "array with hex size",
			input:    "[0x10]byte",
			expected: &ArrayType{Size: 16, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:     "array with octal size",
			input:    "[010]byte",
			expected: &ArrayType{Size: 8, Elem: &NamedType{Name: "byte"}},
		},
		{
			name:        "array with invalid size",
			input:       "[abc]string",
			expectError: true,
		},
		{
			name:        "array with negative size",
			input:       "[-5]string",
			expectError: true,
		},
		{
			name:        "array with float size",
			input:       "[3.14]string",
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
