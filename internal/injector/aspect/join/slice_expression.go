// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"errors"
	"fmt"

	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/yaml"
)

type (
	// SliceExpressionOperand identifies the kind of operand a `slice-expression`
	// join point matches.
	SliceExpressionOperand int

	sliceExpression struct {
		Operand SliceExpressionOperand
	}
)

const (
	// SliceExpressionOperandString matches slice expressions applied to an operand
	// whose core type is exactly `string`.
	SliceExpressionOperandString SliceExpressionOperand = iota
	// SliceExpressionOperandBytes matches slice expressions applied to an operand
	// whose core type is exactly `[]byte`.
	SliceExpressionOperandBytes
)

// SliceExpression matches slice expressions (`x[low:high]` and, for `bytes`,
// `x[low:high:max]`) applied to an operand of the designated kind.
func SliceExpression(operand SliceExpressionOperand) *sliceExpression {
	return &sliceExpression{Operand: operand}
}

func (*sliceExpression) ImpliesImported() []string {
	return nil
}

func (*sliceExpression) PackageMayMatch(*may.PackageContext) may.MatchType {
	// A slice expression does not imply any import.
	return may.Unknown
}

func (*sliceExpression) FileMayMatch(ctx *may.FileContext) may.MatchType {
	// A slice expression necessarily involves the `[` token.
	return ctx.FileContains("[")
}

func (s *sliceExpression) Matches(ctx context.AspectContext) bool {
	expr, ok := ctx.Node().(*dst.SliceExpr)
	if !ok {
		return false
	}

	operandType := ctx.ResolveType(expr.X)
	switch s.Operand {
	case SliceExpressionOperandString:
		// A full slice expression (`x[low:high:max]`) is not valid on a string, so
		// this is rejected defensively.
		if expr.Max != nil {
			return false
		}
		return typed.IsStringCore(operandType)
	case SliceExpressionOperandBytes:
		// Full slice expressions are supported on `[]byte` operands.
		return typed.IsByteSliceCore(operandType)
	default:
		return false
	}
}

func (s *sliceExpression) Hash(h *fingerprint.Hasher) error {
	return h.Named("slice-expression", s.Operand)
}

func init() {
	unmarshalers["slice-expression"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		spec := struct {
			Operand *SliceExpressionOperand `yaml:"operand"`
		}{}
		if err := yaml.NodeToValueContext(ctx, node, &spec); err != nil {
			return nil, err
		}

		if spec.Operand == nil {
			return nil, errors.New("slice-expression: missing required field 'operand'")
		}

		return SliceExpression(*spec.Operand), nil
	}
}

var _ yaml.NodeUnmarshalerContext = (*SliceExpressionOperand)(nil)

func (s *SliceExpressionOperand) UnmarshalYAML(ctx gocontext.Context, node ast.Node) error {
	var name string
	if err := yaml.NodeToValueContext(ctx, node, &name); err != nil {
		return err
	}

	switch name {
	case "string":
		*s = SliceExpressionOperandString
	case "bytes":
		*s = SliceExpressionOperandBytes
	default:
		return fmt.Errorf("invalid slice-expression.operand value: %q", name)
	}

	return nil
}

func (s SliceExpressionOperand) String() string {
	switch s {
	case SliceExpressionOperandString:
		return "string"
	case SliceExpressionOperandBytes:
		return "bytes"
	default:
		return fmt.Sprintf("invalid(%d)", int(s))
	}
}

func (s SliceExpressionOperand) Hash(h *fingerprint.Hasher) error {
	return h.Named("slice-expression-operand", fingerprint.Int(s))
}
