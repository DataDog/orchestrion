// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package orchestrion

import (
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// typeChecker leverages dst/decorator and go/types to infer the expression types.
// The decorator keeps the mapping between ast.Node and dst.Node while go/types extracts the types.
// Call check() after instanciation and before calling ofType() or typeOf().
type typeChecker struct {
	dec  *decorator.Decorator
	info *types.Info
}

// newTypeChecker constructs a typeChecker.
func newTypeChecker(dec *decorator.Decorator) *typeChecker {
	return &typeChecker{
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
func (tc *typeChecker) check(name string, fset *token.FileSet, file *ast.File) {
	conf := &types.Config{
		Importer: importer.Default(),
		Error:    func(err error) { /* ignore type check errors */ },
	}
	_, _ = conf.Check(name, fset, []*ast.File{file}, tc.info)
}

// ofType checks the type of an expression.
func (tc typeChecker) ofType(expr dst.Expr, t string) bool {
	return tc.typeOf(expr) == t
}

// typeOf returns the type of an expression.
func (tc typeChecker) typeOf(expr dst.Expr) string {
	astExpr := tc.dec.Ast.Nodes[expr].(ast.Expr)
	to := tc.info.TypeOf(astExpr)
	if to == nil {
		// this was almost certainly an underscore "_"
		return ""
	}
	return to.String()
}
