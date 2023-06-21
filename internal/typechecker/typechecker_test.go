// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typechecker

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/goast"
	"github.com/stretchr/testify/require"
)

func TestTypeChecker(t *testing.T) {
	code := `package main

	import "net/http"

	type custom struct{}
	
	func main() {
		i := 1337
		s := "random"
		var server http.Server
		addr := server.Addr
		var c custom
		cptr := &custom{}
		var invalid invalid
		var iptr *int64
	}
`
	expected := map[string]string{
		"i":       "int",
		"s":       "string",
		"server":  "net/http.Server",
		"addr":    "string",
		"c":       "test.custom",
		"cptr":    "*test.custom",
		"invalid": "invalid type",
		"iptr":    "*int64",
	}
	name := "test"
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, name, strings.NewReader(code), parser.ParseComments)
	require.NoError(t, err)

	dec := decorator.NewDecoratorWithImports(fset, name, goast.New())
	f, err := dec.DecorateFile(astFile)
	require.NoError(t, err)

	tc := New(dec)
	tc.Check(name, fset, astFile)

	checks := 0
	dst.Inspect(f, func(n dst.Node) bool {
		if ident, ok := n.(*dst.Ident); ok && expected[ident.Name] != "" {
			checks++
			require.Equal(t, expected[ident.Name], tc.TypeOf(ident))
			require.True(t, tc.OfType(ident, expected[ident.Name]))
		}
		return true
	})
	require.GreaterOrEqual(t, checks, len(expected))
}
