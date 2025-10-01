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

func TestNamedType(t *testing.T) {
	// Test cases for NewNamedType which now accepts pointer types
	testCases := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
		expected    *NamedType
	}{
		{
			name:        "invalid syntax",
			input:       "0",
			expectError: true,
			errorMsg:    `invalid type syntax: "0"`,
		},
		{
			name:        "simple type",
			input:       "net/http.ResponseWriter",
			expectError: false,
			expected:    &NamedType{Path: "net/http", Name: "ResponseWriter"},
		},
		{
			name:        "pointer type",
			input:       "*net/http.Request",
			expectError: false,
			expected:    &NamedType{Path: "net/http", Name: "Request"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NewNamedType(tc.input)
			if tc.expectError {
				require.Error(t, err)
				require.EqualError(t, err, tc.errorMsg)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, result)
			}
		})

		t.Run("Must="+tc.name, func(t *testing.T) {
			if tc.expectError {
				require.Panics(t, func() {
					_ = MustNamedType(tc.input)
				})
			} else {
				require.NotPanics(t, func() {
					result := MustNamedType(tc.input)
					require.Equal(t, tc.expected, result)
				})
			}
		})
	}
}

func TestNamedType_Matches(t *testing.T) {
	tests := []struct {
		name     string
		nt       *NamedType
		node     dst.Expr
		expected bool
	}{
		// --- Ident Cases ---
		{
			name:     "MyType matches MyType",
			nt:       MustNamedType("MyType"),
			node:     dst.NewIdent("MyType"),
			expected: true,
		},
		{
			name:     "MyType does not match OtherType",
			nt:       MustNamedType("MyType"),
			node:     dst.NewIdent("OtherType"),
			expected: false,
		},
		{
			name:     "some/path.MyType does not match MyType",
			nt:       MustNamedType("some/path.MyType"),
			node:     dst.NewIdent("MyType"), // node.Path is ""
			expected: false,
		},
		{
			name:     "foo/bar.MyType matches foo/bar.MyType",
			nt:       MustNamedType("foo/bar.MyType"),
			node:     &dst.Ident{Name: "MyType", Path: "foo/bar"},
			expected: true,
		},
		{
			name:     "foo/bar.MyType does not match wrong/path.MyType",
			nt:       MustNamedType("foo/bar.MyType"),
			node:     &dst.Ident{Name: "MyType", Path: "wrong/path"},
			expected: false,
		},

		// --- StarExpr Cases (Pointers) ---
		{
			name:     "MyType does not match *MyType",
			nt:       MustNamedType("MyType"),
			node:     &dst.StarExpr{X: dst.NewIdent("MyType")},
			expected: false,
		},

		// --- SelectorExpr Cases (Qualified types like pkg.Type) ---
		// Note: SelectorExpr is used when the package is imported with an alias
		// For example: import mypkg "some/package" results in mypkg.Type
		{
			name:     "mypkg (alias for some/pkg) matches mypkg.MyType",
			nt:       &NamedType{Path: "mypkg", Name: "MyType"},
			node:     &dst.SelectorExpr{X: dst.NewIdent("mypkg"), Sel: dst.NewIdent("MyType")},
			expected: true,
		},
		{
			name:     "full/import/path.MyType does not match alias.MyType",
			nt:       MustNamedType("full/import/path.MyType"),
			node:     &dst.SelectorExpr{X: dst.NewIdent("alias"), Sel: dst.NewIdent("MyType")},
			expected: false,
		},
		{
			name:     "alias for pkg does not match otheralias.MyType",
			nt:       &NamedType{Path: "pkg", Name: "MyType"},
			node:     &dst.SelectorExpr{X: dst.NewIdent("otherpkg"), Sel: dst.NewIdent("MyType")},
			expected: false,
		},
		{
			name:     "alias.MyType does not match alias.OtherType",
			nt:       &NamedType{Path: "pkg", Name: "MyType"},
			node:     &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("OtherType")},
			expected: false,
		},
		{
			name:     "MyType matches .MyType",
			nt:       MustNamedType("MyType"),
			node:     &dst.SelectorExpr{X: dst.NewIdent(""), Sel: dst.NewIdent("MyType")},
			expected: true,
		},

		// --- InterfaceType Cases (any) ---
		{
			name:     "any matches any",
			nt:       MustNamedType("any"),
			node:     &dst.InterfaceType{Methods: &dst.FieldList{}},
			expected: true,
		},
		{
			name:     "foo/bar.any does not match any",
			nt:       MustNamedType("foo/bar.any"),
			node:     &dst.InterfaceType{Methods: &dst.FieldList{}},
			expected: false,
		},

		// --- IndexExpr Cases (Generics with one type parameter, e.g., MyType[T]) ---
		{
			name:     "MyType matches MyType[T]",
			nt:       MustNamedType("MyType"),
			node:     &dst.IndexExpr{X: dst.NewIdent("MyType"), Index: dst.NewIdent("T")},
			expected: true,
		},
		{
			name:     "OtherType does not match MyType[T]",
			nt:       MustNamedType("OtherType"),
			node:     &dst.IndexExpr{X: dst.NewIdent("MyType"), Index: dst.NewIdent("T")},
			expected: false,
		},
		{
			name:     "foo/bar.MyType matches foo/bar.MyType[T]",
			nt:       MustNamedType("foo/bar.MyType"),
			node:     &dst.IndexExpr{X: &dst.Ident{Name: "MyType", Path: "foo/bar"}, Index: dst.NewIdent("T")},
			expected: true,
		},
		{
			name:     "foo/bar.MyType does not match wrong/path.MyType[T]",
			nt:       MustNamedType("foo/bar.MyType"),
			node:     &dst.IndexExpr{X: &dst.Ident{Name: "MyType", Path: "wrong/path"}, Index: dst.NewIdent("T")},
			expected: false,
		},
		{
			name:     "alias.MyType matches alias.MyType[T]",
			nt:       &NamedType{Path: "pkg", Name: "MyType"},
			node:     &dst.IndexExpr{X: &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")}, Index: dst.NewIdent("T")},
			expected: true,
		},
		{
			name:     "full/import/path.MyType does not match pkg.MyType[T]",
			nt:       MustNamedType("full/import/path.MyType"),
			node:     &dst.IndexExpr{X: &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")}, Index: dst.NewIdent("T")},
			expected: false,
		},
		{
			name:     "MyType does not match (*MyType)[T]",
			nt:       MustNamedType("MyType"),
			node:     &dst.IndexExpr{X: &dst.StarExpr{X: dst.NewIdent("MyType")}, Index: dst.NewIdent("T")},
			expected: false,
		},

		// --- IndexListExpr Cases (Generics with 2+ type parameters, e.g., MyType[T1, T2]) ---
		{
			name:     "MyType matches MyType[T1, T2]",
			nt:       MustNamedType("MyType"),
			node:     &dst.IndexListExpr{X: dst.NewIdent("MyType"), Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: true,
		},
		{
			name:     "OtherType does not match MyType[T1, T2]",
			nt:       MustNamedType("OtherType"),
			node:     &dst.IndexListExpr{X: dst.NewIdent("MyType"), Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: false,
		},
		{
			name:     "foo/bar.MyType matches foo/bar.MyType[T1, T2]",
			nt:       MustNamedType("foo/bar.MyType"),
			node:     &dst.IndexListExpr{X: &dst.Ident{Name: "MyType", Path: "foo/bar"}, Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: true,
		},
		{
			name:     "alias.MyType matches alias.MyType[T1, T2]",
			nt:       &NamedType{Path: "pkg", Name: "MyType"},
			node:     &dst.IndexListExpr{X: &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")}, Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: true,
		},
		{
			name:     "MyType does not match (*MyType)[T1, T2]",
			nt:       MustNamedType("MyType"),
			node:     &dst.IndexListExpr{X: &dst.StarExpr{X: dst.NewIdent("MyType")}, Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: false,
		},

		// --- Unhandled Type Case ---
		{
			name:     "MyType does not match <BasicLit>",
			nt:       MustNamedType("MyType"),
			node:     &dst.BasicLit{},
			expected: false,
		},
		{
			name:     "MyType does not match <nil>",
			nt:       MustNamedType("MyType"),
			node:     nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.nt.Matches(tc.node))
		})
	}
}

func TestNewNamedTypePointerHandling(t *testing.T) {
	testCases := []struct {
		name    string
		typeStr string
		isPtr   bool
	}{
		{
			name:    "value type",
			typeStr: "string",
			isPtr:   false,
		},
		{
			name:    "pointer type",
			typeStr: "*string",
			isPtr:   true,
		},
		{
			name:    "qualified pointer type",
			typeStr: "*kafkatrace.Tracer",
			isPtr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("NewNamedType", func(t *testing.T) {
				namedType, err := NewNamedType(tc.typeStr)
				require.NoError(t, err)

				node := namedType.AsNode()

				_, isStarExpr := node.(*dst.StarExpr)
				require.False(t, isStarExpr,
					"NewNamedType(%s) should strip pointer info but got StarExpr", tc.typeStr)
			})

			t.Run("NewType", func(t *testing.T) {
				typeExpr, err := NewType(tc.typeStr)
				require.NoError(t, err)

				node := typeExpr.AsNode()

				_, isStarExpr := node.(*dst.StarExpr)
				if tc.isPtr {
					require.True(t, isStarExpr,
						"NewType(%s) should preserve pointer info", tc.typeStr)
				} else {
					require.False(t, isStarExpr,
						"NewType(%s) should not be a pointer", tc.typeStr)
				}
			})
		})
	}
}
