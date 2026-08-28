// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"fmt"

	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/concat"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/yaml"
)

const (
	// MinStringConcatOperands is the smallest operand count a `string-concat` join
	// point may be configured with. A concatenation always has at least two
	// operands.
	MinStringConcatOperands = 2
	// MaxStringConcatOperands is the largest operand count a `string-concat` join
	// point may be configured with.
	MaxStringConcatOperands = 16
)

type stringConcat struct {
	// MinOperands is the smallest number of operands a chain must have to match.
	MinOperands int
	// MaxOperands is the largest number of operands a chain may have to match.
	MaxOperands int
}

// StringConcat matches the root of a maximal, non-constant `string`
// concatenation chain having between minOperands and maxOperands operands
// (inclusive).
//
// Both bounds must be between [MinStringConcatOperands] and
// [MaxStringConcatOperands], and minOperands must not be greater than
// maxOperands.
func StringConcat(minOperands int, maxOperands int) (*stringConcat, error) {
	if minOperands < MinStringConcatOperands || minOperands > MaxStringConcatOperands {
		return nil, fmt.Errorf(
			"string-concat: min-operands must be between %d and %d (got %d)",
			MinStringConcatOperands, MaxStringConcatOperands, minOperands,
		)
	}
	if maxOperands < MinStringConcatOperands || maxOperands > MaxStringConcatOperands {
		return nil, fmt.Errorf(
			"string-concat: max-operands must be between %d and %d (got %d)",
			MinStringConcatOperands, MaxStringConcatOperands, maxOperands,
		)
	}
	if minOperands > maxOperands {
		return nil, fmt.Errorf(
			"string-concat: min-operands (%d) must not be greater than max-operands (%d)",
			minOperands, maxOperands,
		)
	}

	return &stringConcat{MinOperands: minOperands, MaxOperands: maxOperands}, nil
}

func (*stringConcat) ImpliesImported() []string {
	return nil
}

func (*stringConcat) PackageMayMatch(*may.PackageContext) may.MatchType {
	// A string concatenation does not imply any import.
	return may.Unknown
}

func (*stringConcat) FileMayMatch(ctx *may.FileContext) may.MatchType {
	// A concatenation expression necessarily involves the `+` token.
	return ctx.FileContains("+")
}

func (s *stringConcat) Matches(ctx context.AspectContext) bool {
	// Only ever match the root of a maximal chain, so that the whole
	// concatenation is advised as the single operation the compiler performs.
	if _, ok := ctx.Node().(*dst.BinaryExpr); !ok {
		return false
	}
	root := concat.Root(ctx)
	if root == nil || root != ctx.Node() {
		return false
	}

	operands := len(concat.Flatten(ctx, root))
	return operands >= s.MinOperands && operands <= s.MaxOperands
}

func (s *stringConcat) Hash(h *fingerprint.Hasher) error {
	return h.Named("string-concat", fingerprint.Int(s.MinOperands), fingerprint.Int(s.MaxOperands))
}

func init() {
	unmarshalers["string-concat"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		spec := struct {
			MinOperands *int `yaml:"min-operands"`
			MaxOperands *int `yaml:"max-operands"`
		}{}
		if err := yaml.NodeToValueContext(ctx, node, &spec); err != nil {
			return nil, err
		}

		minOperands := MinStringConcatOperands
		if spec.MinOperands != nil {
			minOperands = *spec.MinOperands
		}
		maxOperands := MaxStringConcatOperands
		if spec.MaxOperands != nil {
			maxOperands = *spec.MaxOperands
		}

		return StringConcat(minOperands, maxOperands)
	}
}
