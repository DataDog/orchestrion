// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"errors"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeName(t *testing.T) {
	for name, err := range map[string]error{
		"0":                          errors.New(`invalid TypeName syntax: "0"`),
		"any":                        nil,
		"net/http.ResponseWriter":    nil,
		"*net/http.Request":          nil,
		"*123domain.com/foo.Bar":     nil,
		"*123domain.com/456/bar.Baz": nil,
	} {
		t.Run(name, func(t *testing.T) {
			_, e := NewTypeName(name)
			if err == nil {
				require.NoError(t, e)
			} else {
				require.EqualError(t, e, err.Error())
			}
		})

		t.Run("Must="+name, func(t *testing.T) {
			defer func() {
				e, _ := recover().(error)
				if err == nil {
					require.NoError(t, e)
				} else {
					require.EqualError(t, e, err.Error())
				}
			}()
			_ = MustTypeName(name)
		})
	}
}

func TestTypeName_Matches(t *testing.T) {
	tests := []struct {
		name     string
		tn       TypeName
		node     dst.Expr
		expected bool
	}{
		// --- Ident Cases ---
		{
			name:     "MyType matches MyType",
			tn:       MustTypeName("MyType"),
			node:     dst.NewIdent("MyType"),
			expected: true,
		},
		{
			name:     "MyType does not match OtherType",
			tn:       MustTypeName("MyType"),
			node:     dst.NewIdent("OtherType"),
			expected: false,
		},
		{
			name:     "*MyType does not match MyType",
			tn:       MustTypeName("*MyType"),
			node:     dst.NewIdent("MyType"),
			expected: false,
		},
		{
			name:     "some/path.MyType does not match MyType",
			tn:       MustTypeName("some/path.MyType"),
			node:     dst.NewIdent("MyType"), // node.Path is ""
			expected: false,
		},
		{
			name:     "foo/bar.MyType matches foo/bar.MyType",
			tn:       MustTypeName("foo/bar.MyType"),
			node:     &dst.Ident{Name: "MyType", Path: "foo/bar"},
			expected: true,
		},
		{
			name:     "foo/bar.MyType does not match wrong/path.MyType",
			tn:       MustTypeName("foo/bar.MyType"),
			node:     &dst.Ident{Name: "MyType", Path: "wrong/path"},
			expected: false,
		},
		{
			name:     "*foo/bar.MyType does not match foo/bar.MyType",
			tn:       MustTypeName("*foo/bar.MyType"),
			node:     &dst.Ident{Name: "MyType", Path: "foo/bar"},
			expected: false,
		},

		// --- StarExpr Cases (Pointers) ---
		{
			name:     "*MyType matches *MyType",
			tn:       MustTypeName("*MyType"),
			node:     &dst.StarExpr{X: dst.NewIdent("MyType")},
			expected: true,
		},
		{
			name:     "MyType does not match *MyType",
			tn:       MustTypeName("MyType"),
			node:     &dst.StarExpr{X: dst.NewIdent("MyType")},
			expected: false,
		},
		{
			name:     "*MyType does not match *OtherType",
			tn:       MustTypeName("*MyType"),
			node:     &dst.StarExpr{X: dst.NewIdent("OtherType")},
			expected: false,
		},
		{
			name:     "*foo/bar.MyType matches *foo/bar.MyType",
			tn:       MustTypeName("*foo/bar.MyType"),
			node:     &dst.StarExpr{X: &dst.Ident{Name: "MyType", Path: "foo/bar"}},
			expected: true,
		},
		{
			name:     "*foo/bar.MyType does not match *wrong/path.MyType",
			tn:       MustTypeName("*foo/bar.MyType"),
			node:     &dst.StarExpr{X: &dst.Ident{Name: "MyType", Path: "wrong/path"}},
			expected: false,
		},
		{
			name:     "*pkg.MyType matches *pkg.MyType",
			tn:       MustTypeName("*pkg.MyType"),
			node:     &dst.StarExpr{X: &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")}},
			expected: true,
		},
		{
			name: "*MyType matches *MyType[T]",
			tn:   MustTypeName("*MyType"),
			node: &dst.StarExpr{
				X: &dst.IndexExpr{X: dst.NewIdent("MyType"), Index: dst.NewIdent("T")},
			},
			expected: true,
		},
		{
			name: "*MyType matches *MyType[T1, T2]",
			tn:   MustTypeName("*MyType"),
			node: &dst.StarExpr{
				X: &dst.IndexListExpr{X: dst.NewIdent("MyType"), Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			},
			expected: true,
		},
		{
			name: "*OtherType does not match *MyType[T]",
			tn:   MustTypeName("*OtherType"),
			node: &dst.StarExpr{
				X: &dst.IndexExpr{X: dst.NewIdent("MyType"), Index: dst.NewIdent("T")},
			},
			expected: false,
		},

		// --- SelectorExpr Cases (Qualified types like pkg.Type) ---
		{
			name:     "pkg.MyType matches pkg.MyType",
			tn:       MustTypeName("pkg.MyType"),
			node:     &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")},
			expected: true,
		},
		{
			name:     "full/import/path.MyType does not match pkg.MyType",
			tn:       MustTypeName("full/import/path.MyType"),
			node:     &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")},
			expected: false,
		},
		{
			name:     "*pkg.MyType does not match pkg.MyType",
			tn:       MustTypeName("*pkg.MyType"),
			node:     &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")},
			expected: false,
		},
		{
			name:     "pkg.MyType does not match otherpkg.MyType",
			tn:       MustTypeName("pkg.MyType"),
			node:     &dst.SelectorExpr{X: dst.NewIdent("otherpkg"), Sel: dst.NewIdent("MyType")},
			expected: false,
		},
		{
			name:     "pkg.MyType does not match pkg.OtherType",
			tn:       MustTypeName("pkg.MyType"),
			node:     &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("OtherType")},
			expected: false,
		},
		{
			name:     "MyType matches .MyType",
			tn:       MustTypeName("MyType"),
			node:     &dst.SelectorExpr{X: dst.NewIdent(""), Sel: dst.NewIdent("MyType")},
			expected: true,
		},

		// --- InterfaceType Cases (any) ---
		{
			name:     "any matches any",
			tn:       MustTypeName("any"),
			node:     &dst.InterfaceType{Methods: &dst.FieldList{}},
			expected: true,
		},
		{
			name:     "foo.any does not match any",
			tn:       MustTypeName("foo.any"),
			node:     &dst.InterfaceType{Methods: &dst.FieldList{}},
			expected: false,
		},

		// --- IndexExpr Cases (Generics with one type parameter, e.g., MyType[T]) ---
		{
			name:     "MyType matches MyType[T]",
			tn:       MustTypeName("MyType"),
			node:     &dst.IndexExpr{X: dst.NewIdent("MyType"), Index: dst.NewIdent("T")},
			expected: true,
		},
		{
			name:     "OtherType does not match MyType[T]",
			tn:       MustTypeName("OtherType"),
			node:     &dst.IndexExpr{X: dst.NewIdent("MyType"), Index: dst.NewIdent("T")},
			expected: false,
		},
		{
			name:     "*MyType does not match MyType[T]",
			tn:       MustTypeName("*MyType"),
			node:     &dst.IndexExpr{X: dst.NewIdent("MyType"), Index: dst.NewIdent("T")},
			expected: false,
		},
		{
			name:     "foo/bar.MyType matches foo/bar.MyType[T]",
			tn:       MustTypeName("foo/bar.MyType"),
			node:     &dst.IndexExpr{X: &dst.Ident{Name: "MyType", Path: "foo/bar"}, Index: dst.NewIdent("T")},
			expected: true,
		},
		{
			name:     "foo/bar.MyType does not match wrong/path.MyType[T]",
			tn:       MustTypeName("foo/bar.MyType"),
			node:     &dst.IndexExpr{X: &dst.Ident{Name: "MyType", Path: "wrong/path"}, Index: dst.NewIdent("T")},
			expected: false,
		},
		{
			name:     "pkg.MyType matches pkg.MyType[T]",
			tn:       MustTypeName("pkg.MyType"),
			node:     &dst.IndexExpr{X: &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")}, Index: dst.NewIdent("T")},
			expected: true,
		},
		{
			name:     "full/import/path.MyType does not match pkg.MyType[T]",
			tn:       MustTypeName("full/import/path.MyType"),
			node:     &dst.IndexExpr{X: &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")}, Index: dst.NewIdent("T")},
			expected: false,
		},
		{
			name:     "MyType does not match (*MyType)[T]",
			tn:       MustTypeName("MyType"),
			node:     &dst.IndexExpr{X: &dst.StarExpr{X: dst.NewIdent("MyType")}, Index: dst.NewIdent("T")},
			expected: false,
		},

		// --- IndexListExpr Cases (Generics with 2+ type parameters, e.g., MyType[T1, T2]) ---
		{
			name:     "MyType matches MyType[T1, T2]",
			tn:       MustTypeName("MyType"),
			node:     &dst.IndexListExpr{X: dst.NewIdent("MyType"), Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: true,
		},
		{
			name:     "OtherType does not match MyType[T1, T2]",
			tn:       MustTypeName("OtherType"),
			node:     &dst.IndexListExpr{X: dst.NewIdent("MyType"), Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: false,
		},
		{
			name:     "*MyType does not match MyType[T1, T2]",
			tn:       MustTypeName("*MyType"),
			node:     &dst.IndexListExpr{X: dst.NewIdent("MyType"), Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: false,
		},
		{
			name:     "foo/bar.MyType matches foo/bar.MyType[T1, T2]",
			tn:       MustTypeName("foo/bar.MyType"),
			node:     &dst.IndexListExpr{X: &dst.Ident{Name: "MyType", Path: "foo/bar"}, Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: true,
		},
		{
			name:     "pkg.MyType matches pkg.MyType[T1, T2]",
			tn:       MustTypeName("pkg.MyType"),
			node:     &dst.IndexListExpr{X: &dst.SelectorExpr{X: dst.NewIdent("pkg"), Sel: dst.NewIdent("MyType")}, Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: true,
		},
		{
			name:     "MyType does not match (*MyType)[T1, T2]",
			tn:       MustTypeName("MyType"),
			node:     &dst.IndexListExpr{X: &dst.StarExpr{X: dst.NewIdent("MyType")}, Indices: []dst.Expr{dst.NewIdent("T1"), dst.NewIdent("T2")}},
			expected: false,
		},

		// --- Unhandled Type Case ---
		{
			name:     "MyType does not match <BasicLit>",
			tn:       MustTypeName("MyType"),
			node:     &dst.BasicLit{},
			expected: false,
		},
		{
			name:     "MyType does not match <nil>",
			tn:       MustTypeName("MyType"),
			node:     nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.tn.Matches(tc.node))
		})
	}
}
