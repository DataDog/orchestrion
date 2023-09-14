// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import "github.com/dave/dst"

func wrapGRPC(stmt *dst.AssignStmt) {
	/*
		//dd:startwrap
		s := grpc.NewServer(opt1, opt2, orchestrion.GRPCStreamServerInterceptor(), orchestrion.GRPCUnaryServerInterceptor())
		//dd:endwrap
	*/

	/*
		//dd:startwrap
		c, err := grpc.Dial(target, opt1, orchestrion.GRPCStreamClientInterceptor(), orchestrion.GRPCUnaryClientInterceptor())
		//dd:endwrap
	*/
	if !(len(stmt.Lhs) <= 2 && len(stmt.Rhs) == 1) {
		return
	}
	wrap := func(targetName string, opts ...string) {
		if fun, ok := stmt.Rhs[0].(*dst.CallExpr); ok {
			if iden, ok := fun.Fun.(*dst.Ident); ok {
				if !(iden.Name == targetName && iden.Path == "google.golang.org/grpc") {
					return
				}
				markAsWrap(stmt)
				for _, opt := range opts {
					fun.Args = append(fun.Args,
						&dst.CallExpr{Fun: &dst.Ident{Name: opt, Path: "github.com/datadog/orchestrion/instrument"}},
					)
				}
			}
		}
	}
	wrap("NewServer", "GRPCStreamServerInterceptor", "GRPCUnaryServerInterceptor")
	wrap("Dial", "GRPCStreamClientInterceptor", "GRPCUnaryClientInterceptor")
}

// unwrapGRPC unwraps grpc server and client, to be used in dst.Inspect.
// Returns true to continue the traversal, false to stop.
func unwrapGRPC(n dst.Node) bool {
	s, ok := n.(*dst.AssignStmt)
	if !ok {
		return true
	}
	ce, ok := s.Rhs[0].(*dst.CallExpr)
	if !ok {
		return true
	}
	cei, ok := ce.Fun.(*dst.Ident)
	if !ok {
		return true
	}
	if cei.Path != "google.golang.org/grpc" || !(cei.Name == "Dial" || cei.Name == "NewServer") || len(ce.Args) == 0 {
		return true
	}
	removeLast := func(args []dst.Expr, targetFunc string) []dst.Expr {
		if len(args) == 0 {
			return args
		}
		lastArg := args[len(args)-1]
		lastArgExp, ok := lastArg.(*dst.CallExpr)
		if !ok {
			return args
		}
		fun, ok := lastArgExp.Fun.(*dst.Ident)
		if !ok {
			return args
		}
		if !(fun.Path == "github.com/datadog/orchestrion/instrument" && fun.Name == targetFunc) {
			return args
		}
		return args[:len(args)-1]
	}
	removable := []string{
		"GRPCUnaryServerInterceptor",
		"GRPCStreamServerInterceptor",
		"GRPCUnaryClientInterceptor",
		"GRPCStreamClientInterceptor",
	}
	for _, opt := range removable {
		ce.Args = removeLast(ce.Args, opt)
	}
	return true
}
