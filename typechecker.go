package orchestrion

import (
	"go/ast"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

type typeChecker struct {
	dec  *decorator.Decorator
	info *types.Info
}

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

func (tc typeChecker) ofType(iden *dst.Ident, t string) bool {
	astIden := tc.dec.Ast.Nodes[iden].(*ast.Ident)
	return tc.info.TypeOf(astIden).String() == t
}
