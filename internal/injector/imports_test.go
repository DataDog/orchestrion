// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/require"
)

func TestFilterExtraneousImports(t *testing.T) {
	qualifiedImport := func(name string) *dst.ImportSpec {
		return &dst.ImportSpec{Name: &dst.Ident{Name: name}}
	}

	for _, tc := range []struct {
		name     string
		in       []*dst.ImportSpec
		expected *dst.ImportSpec
	}{
		{
			name: "simple",
			in: []*dst.ImportSpec{
				qualifiedImport("test"),
			},
			expected: qualifiedImport("test"),
		},
		{
			name: "multiple-qualified",
			in: []*dst.ImportSpec{
				qualifiedImport("test1"),
				qualifiedImport("test2"),
			},
			expected: qualifiedImport("test1"),
		},
		{
			name: "one-qualified-first",
			in: []*dst.ImportSpec{
				qualifiedImport("test1"),
				{Name: &dst.Ident{Name: "_"}},
			},
			expected: qualifiedImport("test1"),
		},
		{
			name: "one-qualified-last",
			in: []*dst.ImportSpec{
				qualifiedImport("test1"),
			},
			expected: qualifiedImport("test1"),
		},
		{
			name: "one-qualified-first-with-nil",
			in: []*dst.ImportSpec{
				qualifiedImport("test1"),
				{Name: nil},
			},
			expected: qualifiedImport("test1"),
		},
		{
			name: "one-qualified-last-with-nil",
			in: []*dst.ImportSpec{
				{Name: nil},
				qualifiedImport("test1"),
			},
			expected: qualifiedImport("test1"),
		},
		{
			name: "complex-1",
			in: []*dst.ImportSpec{
				{Name: nil},
				{Name: &dst.Ident{Name: "_"}},
				qualifiedImport("test1"),
				{Name: nil},
			},
			expected: qualifiedImport("test1"),
		},
		{
			name: "complex-2",
			in: []*dst.ImportSpec{
				{Name: &dst.Ident{Name: "_"}},
				{Name: nil},
				{Name: nil},
			},
			expected: &dst.ImportSpec{Name: &dst.Ident{Name: "_"}},
		},
		{
			name: "complex-3",
			in: []*dst.ImportSpec{
				{Name: &dst.Ident{Name: "_"}},
				{Name: nil},
				{Name: nil},
			},
			expected: &dst.ImportSpec{Name: &dst.Ident{Name: "_"}},
		},
		{
			name: "complex-4",
			in: []*dst.ImportSpec{
				{Name: nil},
				{Name: &dst.Ident{Name: "_"}},
				{Name: nil},
			},
			expected: &dst.ImportSpec{Name: &dst.Ident{Name: "_"}},
		},
		{
			name: "complex-5",
			in: []*dst.ImportSpec{
				{Name: nil},
				qualifiedImport("test1"),
				{Name: &dst.Ident{Name: "_"}},
				{Name: nil},
				qualifiedImport("test2"),
			},
			expected: qualifiedImport("test1"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filterExtraneousImports(map[string][]*dst.ImportSpec{
				"test": tc.in,
			})

			require.Len(t, out, 1)
			for v := range out {
				require.Equal(t, tc.expected, v)
			}
		})
	}
}
