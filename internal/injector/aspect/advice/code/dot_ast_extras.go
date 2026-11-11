// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import "github.com/dave/dst"

// Name returns the name of the selector's identifier (Sel.Name).
//
// This allows templates to use {{ .AST.Fun.Name }} on both *dst.Ident
// (which exposes Name as a promoted field from the embedded *dst.Ident)
// and *dst.SelectorExpr (which has Sel.Name but no direct Name field).
//
// For a qualified call like http.Get(...), the Fun field is a *dst.SelectorExpr
// where Sel is the *dst.Ident for "Get". This method makes .Name resolve to
// "Get" in that case, consistent with how .Name works on *proxyIdent.
//
// NOTE: This file is a hand-written companion to the auto-generated
// dot_ast.proxies.go. Any future hand-written extensions to generated proxy
// types should be added here to keep them separate from generated code.
func (p *proxySelectorExpr) Name() string {
	return p.SelectorExpr.Sel.Name
}

// Name returns the operand identifier name of an indexed callee. For an
// explicit generic instantiation, such as slices.Clone[[]int](xs), this is the
// function name ("Clone"). For ordinary value indexes, such as Handlers[0](),
// this is the variable name ("Handlers"). It returns an empty string when the
// operand is not an identifier or selector, because there is no meaningful
// name to expose. The name never includes a package qualification or the
// index (type) arguments.
func (p *proxyIndexExpr) Name() string {
	return indexedName(p.IndexExpr.X)
}

func (p *proxyIndexListExpr) Name() string {
	return indexedName(p.IndexListExpr.X)
}

func indexedName(expr dst.Expr) string {
	switch expr := expr.(type) {
	case *dst.Ident:
		return expr.Name
	case *dst.SelectorExpr:
		return expr.Sel.Name
	default:
		return ""
	}
}
