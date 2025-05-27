// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func TestPointerType_Matches(t *testing.T) {
	tests := []struct {
		name     string
		pointer  *PointerType
		node     dst.Expr
		expected bool
	}{
		{
			name:     "matches pointer to string",
			pointer:  &PointerType{Elem: &NamedType{Name: "string"}},
			node:     &dst.StarExpr{X: dst.NewIdent("string")},
			expected: true,
		},
		{
			name:     "does not match non-pointer",
			pointer:  &PointerType{Elem: &NamedType{Name: "string"}},
			node:     dst.NewIdent("string"),
			expected: false,
		},
		{
			name:     "does not match pointer to different type",
			pointer:  &PointerType{Elem: &NamedType{Name: "string"}},
			node:     &dst.StarExpr{X: dst.NewIdent("int")},
			expected: false,
		},
		{
			name:     "matches pointer to qualified type",
			pointer:  &PointerType{Elem: &NamedType{ImportPath: "fmt", Name: "Stringer"}},
			node:     &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("fmt"), Sel: dst.NewIdent("Stringer")}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pointer.Matches(tt.node)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPointerType_AsNode(t *testing.T) {
	tests := []struct {
		name     string
		pointer  *PointerType
		expected dst.Expr
	}{
		{
			name:    "pointer to simple type",
			pointer: &PointerType{Elem: &NamedType{Name: "string"}},
			expected: &dst.StarExpr{X: &dst.Ident{
				Name: "string",
			}},
		},
		{
			name:    "pointer to qualified type",
			pointer: &PointerType{Elem: &NamedType{ImportPath: "fmt", Name: "Stringer"}},
			expected: &dst.StarExpr{X: &dst.Ident{
				Name: "Stringer",
				Path: "fmt",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pointer.AsNode()
			assert.Equal(t, tt.expected, result)
		})
	}
}
