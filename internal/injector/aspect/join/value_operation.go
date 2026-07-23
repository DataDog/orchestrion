// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"fmt"
	"go/token"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type valueOperation string

const (
	stringConcat      valueOperation = "string-concat"
	stringToBytes     valueOperation = "string-to-bytes"
	bytesToString     valueOperation = "bytes-to-string"
	stringSlice       valueOperation = "string-slice"
	bytesSlice        valueOperation = "bytes-slice"
	appendBytes       valueOperation = "append-bytes"
	appendValues      valueOperation = "append-byte-values"
	appendString      valueOperation = "append-string-bytes"
	copyBytes         valueOperation = "copy-bytes"
	copyString        valueOperation = "copy-string-to-bytes"
	clearBytes        valueOperation = "clear-bytes"
	setByte           valueOperation = "set-byte"
	stringByteAt      valueOperation = "string-byte-at"
	bytesByteAt       valueOperation = "bytes-byte-at"
	setFromString     valueOperation = "set-byte-from-string"
	setFromBytes      valueOperation = "set-byte-from-bytes"
	stringToRunes     valueOperation = "string-to-runes"
	runesToString     valueOperation = "runes-to-string"
	captureStringByte valueOperation = "capture-string-byte"
	captureSliceByte  valueOperation = "capture-slice-byte"
	captureSliceRune  valueOperation = "capture-slice-rune"
	byteScalarString  valueOperation = "byte-scalar-to-string"
	runeScalarString  valueOperation = "rune-scalar-to-string"
)

func (valueOperation) ImpliesImported() []string {
	return nil
}

func (valueOperation) PackageMayMatch(*may.PackageContext) may.MatchType {
	return may.Unknown
}

func (operation valueOperation) FileMayMatch(ctx *may.FileContext) may.MatchType {
	switch operation {
	case stringConcat:
		return ctx.FileContains("+")
	case stringToBytes, bytesToString, stringByteAt, bytesByteAt, stringToRunes, runesToString:
		return may.Unknown
	case stringSlice, bytesSlice:
		return ctx.FileContains("[")
	case appendBytes, appendValues, appendString:
		return ctx.FileContains("append")
	case copyBytes, copyString:
		return ctx.FileContains("copy")
	case clearBytes:
		return ctx.FileContains("clear")
	case setByte, setFromString, setFromBytes:
		return ctx.FileContains("=")
	case captureStringByte, captureSliceByte, captureSliceRune, byteScalarString, runeScalarString:
		return may.Unknown
	default:
		return may.NeverMatch
	}
}

func (operation valueOperation) Matches(ctx context.AspectContext) bool {
	switch operation {
	case stringConcat:
		expression, ok := ctx.Node().(*dst.BinaryExpr)
		return ok && expression.Op == token.ADD && !ctx.IsConstant(expression) && isString(ctx.ResolveType(expression))
	case stringToBytes:
		call, ok := ctx.Node().(*dst.CallExpr)
		return ok && isConversion(ctx, call) && isByteSlice(ctx.ResolveType(call)) && isString(ctx.ResolveType(call.Args[0]))
	case bytesToString:
		call, ok := ctx.Node().(*dst.CallExpr)
		return ok && isConversion(ctx, call) && isString(ctx.ResolveType(call)) && isByteSlice(ctx.ResolveType(call.Args[0]))
	case stringToRunes:
		call, ok := ctx.Node().(*dst.CallExpr)
		return ok && isConversion(ctx, call) && isRuneSlice(ctx.ResolveType(call)) && isString(ctx.ResolveType(call.Args[0]))
	case runesToString:
		call, ok := ctx.Node().(*dst.CallExpr)
		return ok && isConversion(ctx, call) && isString(ctx.ResolveType(call)) && isRuneSlice(ctx.ResolveType(call.Args[0]))
	case stringByteAt:
		call, indexed, ok := indexedStringConversion(ctx)
		return ok && isString(ctx.ResolveType(call)) && isString(ctx.ResolveType(indexed.X))
	case bytesByteAt:
		call, indexed, ok := indexedStringConversion(ctx)
		return ok && isString(ctx.ResolveType(call)) && isByteSlice(ctx.ResolveType(indexed.X))
	case stringSlice:
		expression, ok := explicitSlice(ctx.Node())
		return ok && isString(ctx.ResolveType(expression))
	case bytesSlice:
		expression, ok := explicitSlice(ctx.Node())
		return ok && isByteSlice(ctx.ResolveType(expression))
	case appendBytes:
		call, ok := builtinCall(ctx, ctx.Node(), "append", true)
		return ok && isByteSlice(ctx.ResolveType(call)) && isByteSlice(ctx.ResolveType(call.Args[0])) && isByteSlice(ctx.ResolveType(call.Args[1]))
	case appendValues:
		call, ok := builtinCallAtLeast(ctx, ctx.Node(), "append", false, 2)
		if !ok || !isByteSlice(ctx.ResolveType(call)) || !isByteSlice(ctx.ResolveType(call.Args[0])) {
			return false
		}
		for _, argument := range call.Args[1:] {
			if !isByte(ctx.ResolveType(argument)) {
				return false
			}
		}
		return true
	case appendString:
		call, ok := builtinCall(ctx, ctx.Node(), "append", true)
		return ok && isByteSlice(ctx.ResolveType(call)) && isByteSlice(ctx.ResolveType(call.Args[0])) && isString(ctx.ResolveType(call.Args[1]))
	case copyBytes:
		call, ok := builtinCall(ctx, ctx.Node(), "copy", false)
		return ok && isByteSlice(ctx.ResolveType(call.Args[0])) && isByteSlice(ctx.ResolveType(call.Args[1]))
	case copyString:
		call, ok := builtinCall(ctx, ctx.Node(), "copy", false)
		return ok && isByteSlice(ctx.ResolveType(call.Args[0])) && isString(ctx.ResolveType(call.Args[1]))
	case clearBytes:
		call, ok := unaryBuiltinCall(ctx, ctx.Node(), "clear")
		return ok && isByteSlice(ctx.ResolveType(call.Args[0]))
	case setByte:
		assignment, target, ok := byteAssignment(ctx)
		if !ok || !isByte(ctx.ResolveType(assignment.Rhs[0])) {
			return false
		}
		if source, indexed := assignment.Rhs[0].(*dst.IndexExpr); indexed &&
			(isString(ctx.ResolveType(source.X)) || isByteSlice(ctx.ResolveType(source.X))) {
			return false
		}
		return isByteSlice(ctx.ResolveType(target.X))
	case setFromString:
		assignment, target, source, ok := indexedByteAssignment(ctx)
		return ok && isByteSlice(ctx.ResolveType(target.X)) && isString(ctx.ResolveType(source.X)) && isByte(ctx.ResolveType(assignment.Rhs[0]))
	case setFromBytes:
		assignment, target, source, ok := indexedByteAssignment(ctx)
		return ok && isByteSlice(ctx.ResolveType(target.X)) && isByteSlice(ctx.ResolveType(source.X)) && isByte(ctx.ResolveType(assignment.Rhs[0]))
	case captureStringByte:
		_, _, source, ok := scalarExtraction(ctx)
		return ok && isString(ctx.ResolveType(source.X))
	case captureSliceByte:
		_, _, source, ok := scalarExtraction(ctx)
		return ok && isByteSlice(ctx.ResolveType(source.X))
	case captureSliceRune:
		_, _, source, ok := scalarExtraction(ctx)
		return ok && isRuneSlice(ctx.ResolveType(source.X))
	case byteScalarString:
		call, source, ok := scalarStringConversion(ctx)
		return ok && isString(ctx.ResolveType(call)) && isByte(ctx.ResolveType(source))
	case runeScalarString:
		call, source, ok := scalarStringConversion(ctx)
		return ok && isString(ctx.ResolveType(call)) && isRune(ctx.ResolveType(source))
	default:
		return false
	}
}

func (operation valueOperation) Hash(h *fingerprint.Hasher) error {
	return h.Named("value-operation", fingerprint.String(operation))
}

func init() {
	unmarshalers["value-operation"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var operation valueOperation
		if err := yaml.NodeToValueContext(ctx, node, &operation); err != nil {
			return nil, err
		}
		switch operation {
		case stringConcat, stringToBytes, bytesToString, stringSlice, bytesSlice,
			appendBytes, appendValues, appendString, copyBytes, copyString, clearBytes, setByte,
			stringByteAt, bytesByteAt, setFromString, setFromBytes, stringToRunes, runesToString,
			captureStringByte, captureSliceByte, captureSliceRune, byteScalarString, runeScalarString:
			return operation, nil
		default:
			return nil, fmt.Errorf("invalid value-operation %q", operation)
		}
	}
}
