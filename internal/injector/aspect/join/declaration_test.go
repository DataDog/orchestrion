// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
)

// declarationContext is the part of [context.AspectContext] that the declaration-of join point reads.
type declarationContext struct {
	context.AspectContext
	chain      *context.NodeChain
	importPath string
}

func (c declarationContext) Chain() *context.NodeChain { return c.chain }
func (c declarationContext) Node() dst.Node            { return c.chain.Node() }
func (c declarationContext) ImportPath() string        { return c.importPath }

func TestDeclarationOf(t *testing.T) {
	const src = `package pkg

type T struct{}

var Name = 1

const Limit = 2

var (
	First, Second = 3, 4
)

func (T) Method() {}

func (*T) Other() {}

func Function() {
	var Local = 5
	const LocalConst = 6
	_, _ = Local, LocalConst
	{
		var Name = 7
		_ = Name
	}
	_ = func() {
		var Limit = 8
		_ = Limit
	}
}

// Same is both a top-level function and the name of a method.
func Same() {}

func (T) Same() {}
`

	file, err := decorator.ParseFile(token.NewFileSet(), "pkg.go", src, parser.ParseComments)
	require.NoError(t, err)

	// find describes every node that the join point matches.
	find := func(point *declarationOf) []string {
		var (
			chain   *context.NodeChain
			matches []string
		)
		dstutil.Apply(file,
			func(cursor *dstutil.Cursor) bool {
				if cursor.Node() == nil {
					return false
				}
				chain = chain.Child(cursor)
				return true
			},
			func(*dstutil.Cursor) bool {
				if point.Matches(declarationContext{chain: chain, importPath: "example.com/pkg"}) {
					matches = append(matches, describe(chain.Node()))
				}
				chain = chain.Parent()
				return true
			},
		)
		return matches
	}

	const pkg = "example.com/pkg"
	tests := map[string]struct {
		point *declarationOf
		want  []string
	}{
		"variable":                {DeclarationOf(pkg, "Name"), []string{"value Name"}},
		"grouped variable":        {DeclarationOf(pkg, "Second"), []string{"value First, Second"}},
		"constant":                {DeclarationOf(pkg, "Limit"), []string{"value Limit"}},
		"function":                {DeclarationOf(pkg, "Function"), []string{"func Function"}},
		"function and method":     {DeclarationOf(pkg, "Same"), []string{"func Same"}},
		"value receiver method":   {DeclarationOf(pkg, "Method"), nil},
		"pointer receiver method": {DeclarationOf(pkg, "Other"), nil},
		"local variable":          {DeclarationOf(pkg, "Local"), nil},
		"local constant":          {DeclarationOf(pkg, "LocalConst"), nil},
		"other package":           {DeclarationOf("example.com/other", "Name"), nil},
		"type declaration":        {DeclarationOf(pkg, "T"), nil},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, find(tc.point))
		})
	}
}

func describe(node dst.Node) string {
	switch node := node.(type) {
	case *dst.FuncDecl:
		if node.Recv != nil {
			return "method " + node.Name.Name
		}
		return "func " + node.Name.Name
	case *dst.ValueSpec:
		names := make([]string, len(node.Names))
		for i, name := range node.Names {
			names[i] = name.Name
		}
		return "value " + strings.Join(names, ", ")
	default:
		return "other"
	}
}
