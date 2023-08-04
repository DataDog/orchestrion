// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typechecker

import (
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// TypeChecker leverages dst/decorator and go/types to infer the expression types.
// The decorator keeps the mapping between ast.Node and dst.Node while go/types extracts the types.
// Call check() after instanciation and before calling ofType() or typeOf().
type TypeChecker struct {
	dec  *decorator.Decorator
	info *types.Info
}

// newTypeChecker constructs a typeChecker.
func New(dec *decorator.Decorator) *TypeChecker {
	return &TypeChecker{
		dec: dec,
		info: &types.Info{
			Defs:  make(map[*ast.Ident]types.Object),
			Uses:  make(map[*ast.Ident]types.Object),
			Types: make(map[ast.Expr]types.TypeAndValue),
		},
	}
}

// check analyses a target file and stores object types.
// It must be called at least once before calling ofType or typeOf.
func (tc *TypeChecker) Check(name string, fset *token.FileSet, file *ast.File) {
	conf := &types.Config{
		// FIXME: The default importer is unable to know object types from 3rd party imports.
		// See https://github.com/golang/go/issues/10276 and https://github.com/golang/go/issues/10249#issuecomment-86671707
		// A possible workaround is to use the "source" compiler importer.ForCompiler(fset, "source", nil)
		// However, it increases the execution time of orchestrion considerably (x20).
		Importer: importer.Default(),
		Error:    func(err error) { /* ignore type check errors */ },
	}
	_, _ = conf.Check(name, fset, []*ast.File{file}, tc.info)
}

// ofType checks the type of an expression.
func (tc TypeChecker) OfType(expr dst.Expr, t string) bool {
	return tc.TypeOf(expr) == t
}

// typeOf returns the type of an expression.
func (tc TypeChecker) TypeOf(expr dst.Expr) string {
	astExpr := tc.dec.Ast.Nodes[expr].(ast.Expr)
	to := tc.info.TypeOf(astExpr)
	if to == nil {
		// this was almost certainly an underscore "_"
		return ""
	}
	return to.String()
}
