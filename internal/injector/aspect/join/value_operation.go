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
	stringConcat               valueOperation = "string-concat"
	stringToBytes              valueOperation = "string-to-bytes"
	bytesToString              valueOperation = "bytes-to-string"
	stringSlice                valueOperation = "string-slice"
	bytesSlice                 valueOperation = "bytes-slice"
	appendBytes                valueOperation = "append-bytes"
	appendValues               valueOperation = "append-byte-values"
	appendString               valueOperation = "append-string-bytes"
	copyBytes                  valueOperation = "copy-bytes"
	copyString                 valueOperation = "copy-string-to-bytes"
	clearBytes                 valueOperation = "clear-bytes"
	setByte                    valueOperation = "set-byte"
	stringByteAt               valueOperation = "string-byte-at"
	bytesByteAt                valueOperation = "bytes-byte-at"
	setFromString              valueOperation = "set-byte-from-string"
	setFromBytes               valueOperation = "set-byte-from-bytes"
	stringToRunes              valueOperation = "string-to-runes"
	runesToString              valueOperation = "runes-to-string"
	captureStringByte          valueOperation = "capture-string-byte"
	captureSliceByte           valueOperation = "capture-slice-byte"
	captureSliceRune           valueOperation = "capture-slice-rune"
	byteScalarString           valueOperation = "byte-scalar-to-string"
	runeScalarString           valueOperation = "rune-scalar-to-string"
	scalarByteArgument         valueOperation = "scalar-byte-argument"
	runeFromScalarByteArgument valueOperation = "rune-from-scalar-byte-argument"
	byteScalarCallResult       valueOperation = "byte-scalar-call-result"
	runeScalarCallResult       valueOperation = "rune-scalar-call-result"
	byteScalarMapMake          valueOperation = "byte-scalar-map-make"
	runeScalarMapMake          valueOperation = "rune-scalar-map-make"
	byteScalarMapStore         valueOperation = "byte-scalar-map-store"
	runeScalarMapStore         valueOperation = "rune-scalar-map-store"
	byteScalarMapStoreClean    valueOperation = "byte-scalar-map-store-clean"
	runeScalarMapStoreClean    valueOperation = "rune-scalar-map-store-clean"
	byteScalarMapLoad          valueOperation = "byte-scalar-map-load"
	runeScalarMapLoad          valueOperation = "rune-scalar-map-load"
	byteScalarChannelMake      valueOperation = "byte-scalar-channel-make"
	runeScalarChannelMake      valueOperation = "rune-scalar-channel-make"
	byteScalarChannelSend      valueOperation = "byte-scalar-channel-send"
	runeScalarChannelSend      valueOperation = "rune-scalar-channel-send"
	byteScalarChannelSendClean valueOperation = "byte-scalar-channel-send-clean"
	runeScalarChannelSendClean valueOperation = "rune-scalar-channel-send-clean"
	byteScalarChannelReceive   valueOperation = "byte-scalar-channel-receive"
	runeScalarChannelReceive   valueOperation = "rune-scalar-channel-receive"
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
	case captureStringByte, captureSliceByte, captureSliceRune, byteScalarString, runeScalarString,
		scalarByteArgument, runeFromScalarByteArgument, byteScalarCallResult, runeScalarCallResult,
		byteScalarMapMake, runeScalarMapMake, byteScalarMapStore, runeScalarMapStore,
		byteScalarMapStoreClean, runeScalarMapStoreClean, byteScalarMapLoad, runeScalarMapLoad,
		byteScalarChannelMake, runeScalarChannelMake, byteScalarChannelSend, runeScalarChannelSend,
		byteScalarChannelSendClean, runeScalarChannelSendClean, byteScalarChannelReceive, runeScalarChannelReceive:
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
	case scalarByteArgument:
		return matchesScalarByteArgument(ctx)
	case runeFromScalarByteArgument:
		return matchesRuneFromScalarByteArgument(ctx)
	case byteScalarCallResult:
		return scalarCallResult(ctx, isByte)
	case runeScalarCallResult:
		return scalarCallResult(ctx, isRune)
	case byteScalarMapMake:
		return scalarMapMake(ctx, isExactByte)
	case runeScalarMapMake:
		return scalarMapMake(ctx, isExactRune)
	case byteScalarMapStore:
		return scalarMapStore(ctx, isExactByte, true)
	case runeScalarMapStore:
		return scalarMapStore(ctx, isExactRune, true)
	case byteScalarMapStoreClean:
		return scalarMapStore(ctx, isExactByte, false)
	case runeScalarMapStoreClean:
		return scalarMapStore(ctx, isExactRune, false)
	case byteScalarMapLoad:
		return scalarMapLoad(ctx, isExactByte)
	case runeScalarMapLoad:
		return scalarMapLoad(ctx, isExactRune)
	case byteScalarChannelMake:
		return scalarChannelMake(ctx, isExactByte)
	case runeScalarChannelMake:
		return scalarChannelMake(ctx, isExactRune)
	case byteScalarChannelSend:
		return scalarChannelSend(ctx, isExactByte, true)
	case runeScalarChannelSend:
		return scalarChannelSend(ctx, isExactRune, true)
	case byteScalarChannelSendClean:
		return scalarChannelSend(ctx, isExactByte, false)
	case runeScalarChannelSendClean:
		return scalarChannelSend(ctx, isExactRune, false)
	case byteScalarChannelReceive:
		return scalarChannelReceive(ctx, isExactByte)
	case runeScalarChannelReceive:
		return scalarChannelReceive(ctx, isExactRune)
	default:
		return false
	}
}

func (operation valueOperation) Hash(h *fingerprint.Hasher) error {
	return h.Named("value-operation", fingerprint.String(operation))
}

// allValueOperations is the single source of truth for the supported value operations.
// The YAML unmarshaler below accepts exactly this set, and the `value-operation` enum in
// internal/injector/config/schema.json must list exactly the same names -
// Test_SchemaEnumListsEveryValueOperation fails if the two ever drift apart.
var allValueOperations = []valueOperation{
	stringConcat, stringToBytes, bytesToString, stringToRunes, runesToString,
	stringSlice, bytesSlice, stringByteAt, bytesByteAt,
	appendBytes, appendValues, appendString,
	copyBytes, copyString, clearBytes,
	setByte, setFromString, setFromBytes,
	captureStringByte, captureSliceByte, captureSliceRune,
	byteScalarString, runeScalarString,
	scalarByteArgument, runeFromScalarByteArgument,
	byteScalarCallResult, runeScalarCallResult,
	byteScalarMapMake, runeScalarMapMake,
	byteScalarMapStore, runeScalarMapStore,
	byteScalarMapStoreClean, runeScalarMapStoreClean,
	byteScalarMapLoad, runeScalarMapLoad,
	byteScalarChannelMake, runeScalarChannelMake,
	byteScalarChannelSend, runeScalarChannelSend,
	byteScalarChannelSendClean, runeScalarChannelSendClean,
	byteScalarChannelReceive, runeScalarChannelReceive,
}

var supportedValueOperations = func() map[valueOperation]struct{} {
	supported := make(map[valueOperation]struct{}, len(allValueOperations))
	for _, operation := range allValueOperations {
		supported[operation] = struct{}{}
	}
	return supported
}()

func init() {
	unmarshalers["value-operation"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var operation valueOperation
		if err := yaml.NodeToValueContext(ctx, node, &operation); err != nil {
			return nil, err
		}
		if _, supported := supportedValueOperations[operation]; !supported {
			return nil, fmt.Errorf("invalid value-operation %q", operation)
		}
		return operation, nil
	}
}
