// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveType tests the ResolveType method thoroughly, covering all code paths:
// - nil expression
// - unmapped expression
// - mapping to a non-expression AST node
// - basic types (int, string)
// - custom types (named types)
// - pointer types
// - interface types (error)
// - resolution via Types map
// - resolution via Uses map
func TestResolveType(t *testing.T) {
	// Create a simple source file
	src := `package test

type MyInt int
type MyStruct struct {
	Field int
}

func TestFunc(i int, s string, m MyInt, ms *MyStruct) (int, error) {
	return 0, nil
}
`

	// Parse the source file.
	fset := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	require.NoError(t, err)

	// Create the decorator.
	dec := decorator.NewDecorator(fset)
	f, err := dec.DecorateFile(parsedFile)
	require.NoError(t, err)

	// Create type information.
	info := types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}

	// Create a minimal type-checker package to define types.
	pkg := types.NewPackage("test", "test")

	// Create the basic types we'll need for testing.
	intType := types.Typ[types.Int]
	stringType := types.Typ[types.String]
	myIntType := types.NewNamed(types.NewTypeName(0, pkg, "MyInt", nil), intType, nil)
	myStructType := types.NewNamed(types.NewTypeName(0, pkg, "MyStruct", nil),
		types.NewStruct([]*types.Var{types.NewVar(0, pkg, "Field", intType)}, nil), nil)
	myStructPtrType := types.NewPointer(myStructType)

	// Create the error interface type.
	errorType := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

	// Get the AST nodes we need for the test.
	var funcDecl *dst.FuncDecl
	for _, decl := range f.Decls {
		if fd, ok := decl.(*dst.FuncDecl); ok {
			funcDecl = fd
			break
		}
	}
	require.NotNil(t, funcDecl, "FuncDecl not found")

	// Get parameter expressions for testing.
	params := funcDecl.Type.Params.List
	require.Len(t, params, 4, "Expected 4 parameters")

	// Get result expressions for testing.
	results := funcDecl.Type.Results.List
	require.Len(t, results, 2, "Expected 2 results")

	// Create a mapping from dst nodes to ast nodes.
	// Normally this would be populated by the decorator, but we'll create a minimal version for testing.
	nodeMap := make(map[dst.Node]ast.Node)

	// Create AST versions of our expressions for the nodeMap.
	intParamAst := &ast.Ident{Name: "int"}
	stringParamAst := &ast.Ident{Name: "string"}
	myIntParamAst := &ast.Ident{Name: "MyInt"}
	myStructPtrParamAst := &ast.StarExpr{X: &ast.Ident{Name: "MyStruct"}}
	intResultAst := &ast.Ident{Name: "int"}
	errorResultAst := &ast.Ident{Name: "error"}

	// Create a non-expression node for testing the type assertion failure.
	nonExpressionAst := &ast.BadStmt{}

	// Populate the types map.
	info.Types[intParamAst] = types.TypeAndValue{Type: intType}
	info.Types[stringParamAst] = types.TypeAndValue{Type: stringType}
	info.Types[myIntParamAst] = types.TypeAndValue{Type: myIntType}
	info.Types[myStructPtrParamAst] = types.TypeAndValue{Type: myStructPtrType}
	info.Types[intResultAst] = types.TypeAndValue{Type: intType}

	// Set up the error type using the Uses map (simulating how it works in real code).
	errorObj := types.Universe.Lookup("error")
	info.Uses[errorResultAst] = errorObj

	// Add the mapping from dst nodes to ast nodes.
	nodeMap[params[0].Type] = intParamAst         // int
	nodeMap[params[1].Type] = stringParamAst      // string
	nodeMap[params[2].Type] = myIntParamAst       // MyInt
	nodeMap[params[3].Type] = myStructPtrParamAst // *MyStruct
	nodeMap[results[0].Type] = intResultAst       // int
	nodeMap[results[1].Type] = errorResultAst     // error

	// Map an expression to a non-expression ast node for testing the type assertion.
	nodeMap[&dst.Ident{Name: "nonExprAst"}] = nonExpressionAst

	// Create a minimal node chain for the context.
	chain := &NodeChain{
		node: funcDecl,
	}

	ctx := context{
		NodeChain: chain,
		typeInfo:  info,
		nodeMap:   nodeMap,
	}

	testCases := []struct {
		name     string
		expr     dst.Expr
		expected types.Type
	}{
		{
			name:     "nil expression",
			expr:     nil,
			expected: nil,
		},
		{
			name:     "unmapped expression",
			expr:     &dst.Ident{Name: "nonexistent"},
			expected: nil,
		},
		{
			name:     "mapped to non-expression AST node",
			expr:     &dst.Ident{Name: "nonExprAst"},
			expected: nil,
		},
		{
			name:     "int parameter",
			expr:     params[0].Type,
			expected: intType,
		},
		{
			name:     "string parameter",
			expr:     params[1].Type,
			expected: stringType,
		},
		{
			name:     "custom type parameter (MyInt)",
			expr:     params[2].Type,
			expected: myIntType,
		},
		{
			name:     "pointer type parameter (*MyStruct)",
			expr:     params[3].Type,
			expected: myStructPtrType,
		},
		{
			name:     "int result",
			expr:     results[0].Type,
			expected: intType,
		},
		{
			name:     "error result using Uses map",
			expr:     results[1].Type,
			expected: errorType,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolvedType := ctx.ResolveType(tc.expr)

			if tc.expected == nil {
				assert.Nil(t, resolvedType)
				return
			}
			require.NotNil(t, resolvedType, "Type should not be nil for %s", tc.name)

			// Use types.Identical for robust type comparison, except for interfaces like error
			// where assignability might be more appropriate depending on how the type is resolved.
			if types.IsInterface(tc.expected) {
				assert.True(t, types.AssignableTo(resolvedType, tc.expected.Underlying().(*types.Interface)),
					"Expected resolved type %s to be assignable to %s", resolvedType, tc.expected)
				return
			}

			assert.True(t, types.Identical(tc.expected, resolvedType),
				"Expected type %s, but got %s", tc.expected, resolvedType)
		})
	}
}
