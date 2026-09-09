// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"errors"
	"fmt"
	"go/types"

	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	aspectcontext "github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/yaml"
)

// ConversionType identifies a string or byte-slice conversion endpoint.
type ConversionType int

const (
	ConversionString ConversionType = iota
	ConversionBytes
)

type typeConversion struct{ From, To ConversionType }

// TypeConversion creates a matcher for assignment- or return-context conversions.
func TypeConversion(from, to ConversionType) (*typeConversion, error) {
	if from == to || from < ConversionString || from > ConversionBytes || to < ConversionString || to > ConversionBytes {
		return nil, errors.New("type-conversion requires distinct string and bytes types")
	}
	return &typeConversion{From: from, To: to}, nil
}
func (*typeConversion) ImpliesImported() []string                         { return nil }
func (*typeConversion) PackageMayMatch(*may.PackageContext) may.MatchType { return may.Unknown }
func (*typeConversion) FileMayMatch(*may.FileContext) may.MatchType       { return may.Unknown }
func (conversion *typeConversion) Matches(ctx aspectcontext.AspectContext) bool {
	call, ok := ctx.Node().(*dst.CallExpr)
	if !ok || len(call.Args) != 1 || call.Ellipsis {
		return false
	}
	if !conversion.matches(ctx.ResolveType(call.Args[0]), conversion.From) || !conversion.matches(ctx.ResolveType(call.Fun), conversion.To) {
		return false
	}
	chain := ctx.Chain()
	if chain != nil {
		chain = chain.Parent()
	}
	for chain != nil {
		switch parent := chain.Node().(type) {
		case *dst.ParenExpr:
			chain = chain.Parent()
			continue
		case *dst.ReturnStmt:
			return true
		case *dst.AssignStmt:
			if conversion.To == ConversionBytes {
				return false
			}
			return !assignmentDiscards(parent, call)
		case *dst.ValueSpec:
			if conversion.To == ConversionBytes {
				return false
			}
			return !valueSpecDiscards(parent, call)
		default:
			return false
		}
	}
	return false
}
func assignmentDiscards(assignment *dst.AssignStmt, call *dst.CallExpr) bool {
	for index, expression := range assignment.Rhs {
		if unparenExpression(expression) != call || index >= len(assignment.Lhs) {
			continue
		}
		identifier, ok := assignment.Lhs[index].(*dst.Ident)
		return ok && identifier.Name == "_"
	}
	return false
}

func valueSpecDiscards(spec *dst.ValueSpec, call *dst.CallExpr) bool {
	for index, expression := range spec.Values {
		if unparenExpression(expression) != call || index >= len(spec.Names) {
			continue
		}
		return spec.Names[index].Name == "_"
	}
	return false
}

func unparenExpression(expression dst.Expr) dst.Expr {
	for {
		parenthesized, ok := expression.(*dst.ParenExpr)
		if !ok {
			return expression
		}
		expression = parenthesized.X
	}
}

func (*typeConversion) matches(resolved types.Type, kind ConversionType) bool {
	if kind == ConversionString {
		return typed.IsStringCore(resolved)
	}
	return typed.IsByteSliceCore(resolved)
}
func (conversion *typeConversion) Hash(hasher *fingerprint.Hasher) error {
	return hasher.Named("type-conversion", fingerprint.Int(conversion.From), fingerprint.Int(conversion.To))
}

func init() {
	unmarshalers["type-conversion"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var spec struct{ From, To string }
		if err := yaml.NodeToValueContext(ctx, node, &spec); err != nil {
			return nil, err
		}
		parse := func(value string) (ConversionType, error) {
			switch value {
			case "string":
				return ConversionString, nil
			case "bytes":
				return ConversionBytes, nil
			default:
				return 0, fmt.Errorf("invalid conversion type %q", value)
			}
		}
		from, err := parse(spec.From)
		if err != nil {
			return nil, err
		}
		to, err := parse(spec.To)
		if err != nil {
			return nil, err
		}
		return TypeConversion(from, to)
	}
}
