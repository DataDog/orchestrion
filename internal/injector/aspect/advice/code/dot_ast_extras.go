// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

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
