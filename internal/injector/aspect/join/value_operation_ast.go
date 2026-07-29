// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"go/token"
	"go/types"
	"sync"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

var scalarContainerEligibility sync.Map

func explicitSlice(node dst.Node) (*dst.SliceExpr, bool) {
	expression, ok := node.(*dst.SliceExpr)
	return expression, ok && expression.Low != nil && expression.High != nil && expression.Max == nil
}

func builtinCall(ctx context.AspectContext, node dst.Node, name string, ellipsis bool) (*dst.CallExpr, bool) {
	call, ok := node.(*dst.CallExpr)
	if !ok || call.Ellipsis != ellipsis || len(call.Args) != 2 {
		return nil, false
	}
	function, ok := call.Fun.(*dst.Ident)
	return call, ok && function.Path == "" && function.Name == name && ctx.IsBuiltin(function)
}

func builtinCallAtLeast(ctx context.AspectContext, node dst.Node, name string, ellipsis bool, arguments int) (*dst.CallExpr, bool) {
	call, ok := node.(*dst.CallExpr)
	if !ok || call.Ellipsis != ellipsis || len(call.Args) < arguments {
		return nil, false
	}
	function, ok := call.Fun.(*dst.Ident)
	return call, ok && function.Path == "" && function.Name == name && ctx.IsBuiltin(function)
}

func unaryBuiltinCall(ctx context.AspectContext, node dst.Node, name string) (*dst.CallExpr, bool) {
	call, ok := node.(*dst.CallExpr)
	if !ok || call.Ellipsis || len(call.Args) != 1 {
		return nil, false
	}
	function, ok := call.Fun.(*dst.Ident)
	return call, ok && function.Path == "" && function.Name == name && ctx.IsBuiltin(function)
}

func isConversion(ctx context.AspectContext, call *dst.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	target := ctx.ResolveType(call.Fun)
	if target == nil {
		return false
	}
	_, function := target.(*types.Signature)
	return !function
}

func indexedStringConversion(ctx context.AspectContext) (*dst.CallExpr, *dst.IndexExpr, bool) {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok || !isConversion(ctx, call) || !isString(ctx.ResolveType(call)) {
		return nil, nil, false
	}
	indexed, ok := call.Args[0].(*dst.IndexExpr)
	return call, indexed, ok
}

func byteAssignment(ctx context.AspectContext) (*dst.AssignStmt, *dst.IndexExpr, bool) {
	assignment, ok := ctx.Node().(*dst.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil, nil, false
	}
	target, ok := assignment.Lhs[0].(*dst.IndexExpr)
	return assignment, target, ok
}

func indexedByteAssignment(ctx context.AspectContext) (*dst.AssignStmt, *dst.IndexExpr, *dst.IndexExpr, bool) {
	assignment, target, ok := byteAssignment(ctx)
	if !ok {
		return nil, nil, nil, false
	}
	source, ok := assignment.Rhs[0].(*dst.IndexExpr)
	return assignment, target, source, ok
}

func scalarExtraction(ctx context.AspectContext) (*dst.AssignStmt, *dst.Ident, *dst.IndexExpr, bool) {
	assignment, ok := ctx.Node().(*dst.AssignStmt)
	if !ok || (assignment.Tok != token.DEFINE && assignment.Tok != token.ASSIGN) || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil, nil, nil, false
	}
	parent := ctx.Parent()
	if parent == nil {
		return nil, nil, nil, false
	}
	if _, ok := parent.Node().(*dst.BlockStmt); !ok {
		return nil, nil, nil, false
	}
	for ancestor := parent.Parent(); ancestor != nil; ancestor = ancestor.Parent() {
		switch ancestor.Node().(type) {
		case *dst.ForStmt, *dst.RangeStmt:
			return nil, nil, nil, false
		}
	}
	target, ok := assignment.Lhs[0].(*dst.Ident)
	if !ok || target.Name == "_" {
		return nil, nil, nil, false
	}
	if assignment.Tok == token.ASSIGN && !scalarTargetOwnedByCurrentFunction(ctx, target) {
		return nil, nil, nil, false
	}
	source, ok := assignment.Rhs[0].(*dst.IndexExpr)
	if !ok || !repeatableScalarOperand(ctx, source.X) || !repeatableScalarOperand(ctx, source.Index) {
		return nil, nil, nil, false
	}
	return assignment, target, source, true
}

func scalarTargetOwnedByCurrentFunction(ctx context.AspectContext, target *dst.Ident) bool {
	if target.Obj == nil {
		return false
	}
	declaration, ok := target.Obj.Decl.(dst.Node)
	if !ok {
		return false
	}
	for ancestor := ctx.Chain(); ancestor != nil; ancestor = ancestor.Parent() {
		switch function := ancestor.Node().(type) {
		case *dst.FuncDecl:
			return functionOwnsDeclaration(function.Type, function.Body, declaration)
		case *dst.FuncLit:
			return functionOwnsDeclaration(function.Type, function.Body, declaration)
		}
	}
	return false
}

func functionOwnsDeclaration(functionType *dst.FuncType, body *dst.BlockStmt, declaration dst.Node) bool {
	for _, fields := range []*dst.FieldList{functionType.Params, functionType.Results} {
		if fields == nil {
			continue
		}
		for _, field := range fields.List {
			if field == declaration {
				return true
			}
		}
	}

	owned := false
	dstutil.Apply(body, func(cursor *dstutil.Cursor) bool {
		node := cursor.Node()
		if node != body {
			if _, nested := node.(*dst.FuncLit); nested {
				return false
			}
		}
		if node == declaration {
			owned = true
			return false
		}
		return true
	}, nil)
	return owned
}

func matchesScalarByteArgument(ctx context.AspectContext) bool {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	argument := call.Args[0]
	return ctx.IsAddressable(argument) && isByte(ctx.ResolveType(argument))
}

func matchesRuneFromScalarByteArgument(ctx context.AspectContext) bool {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	conversion, ok := call.Args[0].(*dst.CallExpr)
	if !ok || !isConversion(ctx, conversion) || !isRune(ctx.ResolveType(conversion)) {
		return false
	}
	argument := conversion.Args[0]
	return ctx.IsAddressable(argument) && isByte(ctx.ResolveType(argument))
}

func scalarCallResult(ctx context.AspectContext, matches func(types.Type) bool) bool {
	assignment, target, ok := predeclaredScalarAssignment(ctx)
	if !ok {
		return false
	}
	call, ok := assignment.Rhs[0].(*dst.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	function, ok := call.Fun.(*dst.Ident)
	if !ok || function.Obj == nil || function.Obj.Kind != dst.Fun || !scalarFunctionType(ctx.ResolveType(function), matches) {
		return false
	}
	argument := call.Args[0]
	return target.Name != "_" && ctx.IsAddressable(argument) && matches(ctx.ResolveType(argument))
}

func scalarMapMake(ctx context.AspectContext, matches func(types.Type) bool) bool {
	assignment, target, ok := localScalarAssignment(ctx)
	if !ok {
		return false
	}
	call, ok := assignment.Rhs[0].(*dst.CallExpr)
	if !ok || len(call.Args) != 1 || !isMakeCall(ctx, call) {
		return false
	}
	if target.Name == "_" || !mapElementMatches(ctx.ResolveType(call), matches) || !localContainerUsesCompatible(ctx, target, scalarMapContainer) {
		return false
	}
	scalarContainerEligibility.Store(target.Obj, scalarMapContainer)
	return true
}

func scalarMapStore(ctx context.AspectContext, matches func(types.Type) bool, addressable bool) bool {
	assignment, ok := ctx.Node().(*dst.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return false
	}
	target, ok := assignment.Lhs[0].(*dst.IndexExpr)
	if !ok || !localScalarMap(ctx, target.X, matches) {
		return false
	}
	source := assignment.Rhs[0]
	return ctx.IsAddressable(source) == addressable && (!addressable || matches(ctx.ResolveType(source)))
}

func scalarMapLoad(ctx context.AspectContext, matches func(types.Type) bool) bool {
	assignment, target, ok := predeclaredScalarAssignment(ctx)
	if !ok {
		return false
	}
	source, ok := assignment.Rhs[0].(*dst.IndexExpr)
	return ok && target.Name != "_" && localScalarMap(ctx, source.X, matches)
}

func scalarChannelMake(ctx context.AspectContext, matches func(types.Type) bool) bool {
	assignment, target, ok := localScalarAssignment(ctx)
	if !ok {
		return false
	}
	call, ok := assignment.Rhs[0].(*dst.CallExpr)
	if !ok || len(call.Args) != 2 || !isMakeCall(ctx, call) {
		return false
	}
	if target.Name == "_" || !channelElementMatches(ctx.ResolveType(call), matches) || !localContainerUsesCompatible(ctx, target, scalarChannelContainer) {
		return false
	}
	scalarContainerEligibility.Store(target.Obj, scalarChannelContainer)
	return true
}

func scalarChannelSend(ctx context.AspectContext, matches func(types.Type) bool, addressable bool) bool {
	if insideSelect(ctx) {
		return false
	}
	send, ok := ctx.Node().(*dst.SendStmt)
	return ok && localScalarChannel(ctx, send.Chan, matches) && ctx.IsAddressable(send.Value) == addressable && (!addressable || matches(ctx.ResolveType(send.Value)))
}

func scalarChannelReceive(ctx context.AspectContext, matches func(types.Type) bool) bool {
	if insideSelect(ctx) {
		return false
	}
	assignment, target, ok := predeclaredScalarAssignment(ctx)
	if !ok {
		return false
	}
	receive, ok := assignment.Rhs[0].(*dst.UnaryExpr)
	return ok && receive.Op == token.ARROW && target.Name != "_" && localScalarChannel(ctx, receive.X, matches)
}

func localScalarAssignment(ctx context.AspectContext) (*dst.AssignStmt, *dst.Ident, bool) {
	assignment, ok := ctx.Node().(*dst.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil, nil, false
	}
	target, ok := assignment.Lhs[0].(*dst.Ident)
	return assignment, target, ok && target.Name != "_"
}

func predeclaredScalarAssignment(ctx context.AspectContext) (*dst.AssignStmt, *dst.Ident, bool) {
	assignment, ok := ctx.Node().(*dst.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil, nil, false
	}
	target, ok := assignment.Lhs[0].(*dst.Ident)
	return assignment, target, ok && target.Name != "_" && scalarTargetOwnedByCurrentFunction(ctx, target)
}

func isMakeCall(ctx context.AspectContext, call *dst.CallExpr) bool {
	function, ok := call.Fun.(*dst.Ident)
	return ok && function.Path == "" && function.Name == "make" && ctx.IsBuiltin(function)
}

func scalarFunctionType(value types.Type, matches func(types.Type) bool) bool {
	signature, ok := value.(*types.Signature)
	return ok && signature.Params().Len() == 1 && signature.Results().Len() == 1 && matches(signature.Params().At(0).Type()) && matches(signature.Results().At(0).Type())
}

func localScalarMap(ctx context.AspectContext, expression dst.Expr, matches func(types.Type) bool) bool {
	identifier, ok := expression.(*dst.Ident)
	if !ok || identifier.Obj == nil || !mapElementMatches(ctx.ResolveType(identifier), matches) || !scalarContainerMarked(identifier, scalarMapContainer) {
		return false
	}
	assignment, ok := identifier.Obj.Decl.(*dst.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return false
	}
	call, ok := assignment.Rhs[0].(*dst.CallExpr)
	return ok && len(call.Args) == 1 && isMakeCall(ctx, call)
}

func localScalarChannel(ctx context.AspectContext, expression dst.Expr, matches func(types.Type) bool) bool {
	identifier, ok := expression.(*dst.Ident)
	if !ok || identifier.Obj == nil || !channelElementMatches(ctx.ResolveType(identifier), matches) || !scalarContainerMarked(identifier, scalarChannelContainer) {
		return false
	}
	assignment, ok := identifier.Obj.Decl.(*dst.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return false
	}
	call, ok := assignment.Rhs[0].(*dst.CallExpr)
	return ok && len(call.Args) == 2 && isMakeCall(ctx, call)
}

type scalarContainerKind int

const (
	scalarMapContainer scalarContainerKind = iota
	scalarChannelContainer
)

func scalarContainerMarked(identifier *dst.Ident, kind scalarContainerKind) bool {
	marked, found := scalarContainerEligibility.Load(identifier.Obj)
	return found && marked == kind
}

func localContainerUsesCompatible(ctx context.AspectContext, target *dst.Ident, kind scalarContainerKind) bool {
	if target.Obj == nil {
		return false
	}
	functionType, body := containerOwner(ctx, target)
	if body == nil {
		return false
	}
	parents := make(map[dst.Node]dst.Node)
	dstutil.Apply(body, func(cursor *dstutil.Cursor) bool {
		parents[cursor.Node()] = cursor.Parent()
		return true
	}, nil)
	compatible := true
	declaration, _ := target.Obj.Decl.(*dst.AssignStmt)
	dstutil.Apply(body, func(cursor *dstutil.Cursor) bool {
		identifier, ok := cursor.Node().(*dst.Ident)
		if !ok || identifier.Obj != target.Obj {
			return true
		}
		if declaration != nil && len(declaration.Lhs) == 1 && declaration.Lhs[0] == identifier {
			return true
		}
		if nestedFunctionUse(identifier, body, parents) {
			compatible = false
			return false
		}
		switch kind {
		case scalarMapContainer:
			compatible = compatibleMapUse(identifier, parents, functionType, body)
		case scalarChannelContainer:
			compatible = compatibleChannelUse(identifier, parents, functionType, body)
		}
		return compatible
	}, nil)
	return compatible
}

func containerOwner(ctx context.AspectContext, target *dst.Ident) (*dst.FuncType, *dst.BlockStmt) {
	declaration, ok := target.Obj.Decl.(dst.Node)
	if !ok {
		return nil, nil
	}
	var functionType *dst.FuncType
	var body *dst.BlockStmt
	dstutil.Apply(ctx.File(), func(cursor *dstutil.Cursor) bool {
		if functionType != nil {
			return false
		}
		switch function := cursor.Node().(type) {
		case *dst.FuncDecl:
			if functionOwnsDeclaration(function.Type, function.Body, declaration) {
				functionType, body = function.Type, function.Body
			}
		case *dst.FuncLit:
			if functionOwnsDeclaration(function.Type, function.Body, declaration) {
				functionType, body = function.Type, function.Body
			}
		}
		return true
	}, nil)
	return functionType, body
}

func nestedFunctionUse(node dst.Node, body *dst.BlockStmt, parents map[dst.Node]dst.Node) bool {
	for parent := parents[node]; parent != nil && parent != body; parent = parents[parent] {
		if _, nested := parent.(*dst.FuncLit); nested {
			return true
		}
	}
	return false
}

func compatibleMapUse(identifier *dst.Ident, parents map[dst.Node]dst.Node, functionType *dst.FuncType, body *dst.BlockStmt) bool {
	index, ok := parents[identifier].(*dst.IndexExpr)
	if !ok || index.X != identifier {
		return false
	}
	assignment, ok := parents[index].(*dst.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return false
	}
	if assignment.Lhs[0] == index {
		return true
	}
	target, ok := assignment.Lhs[0].(*dst.Ident)
	return assignment.Rhs[0] == index && target != nil && target.Obj != nil && scalarDeclarationOwnedByFunction(functionType, body, target.Obj.Decl)
}

func compatibleChannelUse(identifier *dst.Ident, parents map[dst.Node]dst.Node, functionType *dst.FuncType, body *dst.BlockStmt) bool {
	if send, ok := parents[identifier].(*dst.SendStmt); ok && send.Chan == identifier {
		return !hasSelectAncestor(send, parents)
	}
	receive, ok := parents[identifier].(*dst.UnaryExpr)
	if !ok || receive.Op != token.ARROW || receive.X != identifier || hasSelectAncestor(receive, parents) {
		return false
	}
	assignment, ok := parents[receive].(*dst.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 || assignment.Rhs[0] != receive {
		return false
	}
	target, ok := assignment.Lhs[0].(*dst.Ident)
	return ok && target.Obj != nil && scalarDeclarationOwnedByFunction(functionType, body, target.Obj.Decl)
}

func hasSelectAncestor(node dst.Node, parents map[dst.Node]dst.Node) bool {
	for parent := parents[node]; parent != nil; parent = parents[parent] {
		if _, selectStatement := parent.(*dst.SelectStmt); selectStatement {
			return true
		}
	}
	return false
}

func scalarDeclarationOwnedByFunction(functionType *dst.FuncType, body *dst.BlockStmt, declaration any) bool {
	node, ok := declaration.(dst.Node)
	return ok && functionOwnsDeclaration(functionType, body, node)
}

func mapElementMatches(value types.Type, matches func(types.Type) bool) bool {
	if value == nil {
		return false
	}
	mapType, ok := value.Underlying().(*types.Map)
	return ok && matches(mapType.Elem())
}

func channelElementMatches(value types.Type, matches func(types.Type) bool) bool {
	if value == nil {
		return false
	}
	channel, ok := value.Underlying().(*types.Chan)
	return ok && matches(channel.Elem())
}

func insideSelect(ctx context.AspectContext) bool {
	for ancestor := ctx.Chain().Parent(); ancestor != nil; ancestor = ancestor.Parent() {
		if _, ok := ancestor.Node().(*dst.SelectStmt); ok {
			return true
		}
	}
	return false
}

func repeatableScalarOperand(ctx context.AspectContext, expression dst.Expr) bool {
	if _, ok := expression.(*dst.Ident); ok {
		return true
	}
	if call, ok := expression.(*dst.CallExpr); ok && isConversion(ctx, call) && len(call.Args) == 1 {
		return repeatableScalarOperand(ctx, call.Args[0])
	}
	return ctx.IsConstant(expression)
}

func scalarStringConversion(ctx context.AspectContext) (*dst.CallExpr, *dst.Ident, bool) {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok || !isConversion(ctx, call) || !isString(ctx.ResolveType(call)) {
		return nil, nil, false
	}
	source, ok := call.Args[0].(*dst.Ident)
	return call, source, ok && ctx.IsAddressable(source)
}
