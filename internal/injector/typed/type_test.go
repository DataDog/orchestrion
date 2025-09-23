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

func TestType(t *testing.T) {
	tests := []struct {
		name     string
		typeStr  string
		expected Type
		wantErr  bool
	}{
		{
			name:     "simple type",
			typeStr:  "string",
			expected: &NamedType{Name: "string"},
		},
		{
			name:     "qualified type",
			typeStr:  "net/http.Request",
			expected: &NamedType{Path: "net/http", Name: "Request"},
		},
		{
			name:     "pointer to simple type",
			typeStr:  "*string",
			expected: &PointerType{Elem: &NamedType{Name: "string"}},
		},
		{
			name:     "pointer to qualified type",
			typeStr:  "*net/http.Request",
			expected: &PointerType{Elem: &NamedType{Path: "net/http", Name: "Request"}},
		},
		{
			name:    "invalid syntax",
			typeStr: "0",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewType(tc.typeStr)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestType_Matches(t *testing.T) {
	tests := []struct {
		name     string
		typ      Type
		node     dst.Expr
		expected bool
	}{
		// NamedType cases
		{
			name:     "named type matches ident",
			typ:      &NamedType{Name: "string"},
			node:     dst.NewIdent("string"),
			expected: true,
		},
		{
			name:     "named type does not match pointer",
			typ:      &NamedType{Name: "string"},
			node:     &dst.StarExpr{X: dst.NewIdent("string")},
			expected: false,
		},
		// PointerType cases
		{
			name:     "pointer type matches pointer",
			typ:      &PointerType{Elem: &NamedType{Name: "string"}},
			node:     &dst.StarExpr{X: dst.NewIdent("string")},
			expected: true,
		},
		{
			name:     "pointer type does not match non-pointer",
			typ:      &PointerType{Elem: &NamedType{Name: "string"}},
			node:     dst.NewIdent("string"),
			expected: false,
		},
		{
			name:     "pointer to qualified type matches",
			typ:      &PointerType{Elem: &NamedType{Path: "net/http", Name: "Request"}},
			node:     &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("net/http"), Sel: dst.NewIdent("Request")}},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.typ.Matches(tc.node))
		})
	}
}
