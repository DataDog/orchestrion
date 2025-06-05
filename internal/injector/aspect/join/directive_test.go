// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectiveMatch(t *testing.T) {
	dir := Directive("test:directive")

	require.True(t, dir.matches("\t//test:directive"))
	require.True(t, dir.matches("\t//test:directive   "))
	require.True(t, dir.matches("\t//test:directive with:arguments"))

	// Not the same directive at all
	require.False(t, dir.matches("\t//test:different"))
	require.False(t, dir.matches("\t//test:directive2"))
	// Not a directive (space after the //)
	require.False(t, dir.matches("\t// test:directive"))
	// Not a directive (not a single-line comment syntax)
	require.False(t, dir.matches("\t/*test:directive*/"))
}

func TestDirective(t *testing.T) {
	type testCase struct {
		preamble      string
		statement     string
		expectMatches []string // The [testCase.statement] is always matched (last), and must not be represented here.
	}
	tests := map[string]testCase{
		"assignment-declaration": {
			statement: "foo := func(...int) {}(1, 2, 3)",
			expectMatches: []string{
				"foo", // matches because it's being defined here
				"func(...int) {}",
				"func(...int) {}(1, 2, 3)",
			},
		},
		"assignment": {
			preamble:  "var foo func(...int)",
			statement: "foo = func(...int) {}(1, 2, 3)",
			expectMatches: []string{
				"func(...int) {}",
				"func(...int) {}(1, 2, 3)",
			},
		},
		"multi-assignment": {
			preamble:  "var foo func(...int)",
			statement: "_, foo = nil, func(...int) {}(1, 2, 3)",
			expectMatches: []string{
				"nil",
				"func(...int) {}",
				"func(...int) {}(1, 2, 3)",
			},
		},
		"call": {
			preamble:  "var foo func(...int)",
			statement: "foo(1, 2, 3)",
			expectMatches: []string{
				"foo",
				"foo(1, 2, 3)", // Quirck -- this is an ExprStmt, so it matches twice (the statement, the expression)
			},
		},
		"immediately-invoked-function-expression": {
			statement: "func(...int) {}(1, 2, 3)",
			expectMatches: []string{
				"func(...int) {}",
				"func(...int) {}(1, 2, 3)", // Quirck -- this is an ExprStmt, so it matches twice (the statement, the expression)
			},
		},
		"defer": {
			statement: "defer func(...int) {}(1, 2, 3)",
			expectMatches: []string{
				"func(...int) {}",
				"func(...int) {}(1, 2, 3)",
			},
		},
		"go": {
			statement: "go func(...int) {}(1, 2, 3)",
			expectMatches: []string{
				"func(...int) {}",
				"func(...int) {}(1, 2, 3)",
			},
		},
		"return": {
			statement: "return func(...int) int { return 0 }(1, 2, 3)",
			expectMatches: []string{
				"func(...int) int { return 0 }",
				"func(...int) int { return 0 }(1, 2, 3)",
			},
		},
		"chan-send": {
			preamble:  "var ch chan <-int",
			statement: "ch <- func(...int) int { return 0 }(1, 2, 3)",
			expectMatches: []string{
				"func(...int) int { return 0 }",
				"func(...int) int { return 0 }(1, 2, 3)",
			},
		},
	}

	const pragma = "//test:directive"
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			source := strings.Join(
				[]string{
					"package main",
					"func main() {",
					tc.preamble,
					pragma,
					tc.statement,
					"}",
				},
				"\n",
			)

			fset := token.NewFileSet()
			astFile, err := parser.ParseFile(fset, "input.go", source, parser.ParseComments)
			require.NoError(t, err)

			dec := decorator.NewDecorator(fset)

			dstFile, err := dec.DecorateFile(astFile)
			require.NoError(t, err)

			visitor := &visitor{pragma: Directive("test:directive")}
			dstutil.Apply(
				dstFile,
				visitor.pre,
				visitor.post,
			)
			require.NotEmpty(t, visitor.matches)

			descriptors := make([]string, len(visitor.matches))
			for idx, match := range visitor.matches {
				node := dec.Ast.Nodes[match.Node()]

				var str bytes.Buffer
				printer.Fprint(&str, fset, node)

				descriptors[idx] = str.String()
			}
			assert.Equal(t, tc.statement, strings.TrimPrefix(descriptors[len(descriptors)-1], pragma+"\n"),
				"the statement itself should always be matched")
			assert.Equal(t, tc.expectMatches, descriptors[:len(descriptors)-1])
		})
	}
}

type visitor struct {
	pragma  directive
	file    *dst.File
	node    *context.NodeChain
	matches []*context.NodeChain
}

func (v *visitor) pre(cursor *dstutil.Cursor) bool {
	if cursor.Node() == nil {
		return false
	}

	if file, ok := cursor.Node().(*dst.File); ok {
		v.file = file
	}

	v.node = v.node.Child(cursor)
	return true
}

func (v *visitor) post(*dstutil.Cursor) bool {
	if v.pragma.matchesChain(v.node) {
		v.matches = append(v.matches, v.node)
	}
	v.node = v.node.Parent()
	return true
}
