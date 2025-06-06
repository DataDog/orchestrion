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
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type structDefinition struct {
	TypeName typed.TypeName
}

// StructDefinition matches the definition of a particular struct given its fully qualified name.
func StructDefinition(typeName typed.TypeName) *structDefinition {
	return &structDefinition{
		TypeName: typeName,
	}
}

func (s *structDefinition) ImpliesImported() []string {
	if path := s.TypeName.ImportPath; path != "" {
		return []string{path}
	}
	return nil
}

func (s *structDefinition) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	if ctx.ImportPath == s.TypeName.ImportPath {
		return may.Match
	}

	return may.NeverMatch
}

func (*structDefinition) FileMayMatch(ctx *may.FileContext) may.MatchType {
	return ctx.FileContains("struct")
}

func (s *structDefinition) Matches(ctx context.AspectContext) bool {
	if s.TypeName.Pointer {
		// We can't ever match a pointer definition
		return false
	}

	spec, ok := ctx.Node().(*dst.TypeSpec)
	if !ok || spec.Name == nil || spec.Name.Name != s.TypeName.Name {
		return false
	}

	if _, ok := spec.Type.(*dst.StructType); !ok {
		return false
	}

	return ctx.ImportPath() == s.TypeName.ImportPath
}

func (s *structDefinition) Hash(h *fingerprint.Hasher) error {
	return h.Named("struct-definition", s.TypeName)
}

type (
	StructLiteralMatch int
	structLiteral      struct {
		TypeName typed.TypeName
		Field    string
		Match    StructLiteralMatch
	}
)

const (
	// StructLiteralMatchAny matches struct literals regardless of whether they are pointer or value.
	// [StructLiteral] join points specified with this match type may match [*dst.CompositeLit] or
	// [*dst.UnaryExpr] nodes.
	StructLiteralMatchAny StructLiteralMatch = iota
	// StructLiteralMatchValueOnly matches struct literals that are not pointers. [StructLiteral] join
	// points specified with this match type only ever match [*dst.CompositeLit] nodes.
	StructLiteralMatchValueOnly
	// StructLiteralMatchPointerOnly matches struct literals that are pointers. [StructLiteral] join
	// points specified with this match type only ever match [*dst.UnaryExpr] nodes.
	StructLiteralMatchPointerOnly
)

// StructLiteralField matches a specific field in struct literals of the designated type.
func StructLiteralField(typeName typed.TypeName, field string) *structLiteral {
	return &structLiteral{
		TypeName: typeName,
		Field:    field,
	}
}

// StructLiteral matches struct literal expressions of the designated type, filtered by the
// specified match type.
func StructLiteral(typeName typed.TypeName, match StructLiteralMatch) *structLiteral {
	return &structLiteral{
		TypeName: typeName,
		Match:    match,
	}
}

func (s *structLiteral) ImpliesImported() []string {
	if path := s.TypeName.ImportPath; path != "" {
		return []string{path}
	}
	return nil
}

func (s *structLiteral) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	return ctx.PackageImports(s.TypeName.ImportPath)
}

func (*structLiteral) FileMayMatch(_ *may.FileContext) may.MatchType {
	return may.Unknown
}

func (s *structLiteral) Matches(ctx context.AspectContext) bool {
	if s.Field == "" {
		switch s.Match {
		case StructLiteralMatchPointerOnly:
			// match only if the current node is equal to & and the underlying node matches
			// the struct literal we are looking for
			if expr, ok := ctx.Node().(*dst.UnaryExpr); ok && expr.Op == token.AND {
				return s.matchesLiteral(expr.X)
			}
			return false

		case StructLiteralMatchValueOnly:
			// do not match if the parent is equal to &
			if parent := ctx.Chain().Parent(); parent != nil {
				if expr, ok := parent.Node().(*dst.UnaryExpr); ok && expr.Op == token.AND {
					return false
				}
			}
			return s.matchesLiteral(ctx.Node())

		default:
			return s.matchesLiteral(ctx.Node())
		}
	}

	kve, ok := ctx.Node().(*dst.KeyValueExpr)
	if !ok {
		return false
	}

	if parent := ctx.Chain().Parent(); parent == nil || !s.matchesLiteral(parent.Node()) {
		return false
	}

	key, ok := kve.Key.(*dst.Ident)
	if !ok {
		return false
	}

	return key.Name == s.Field
}

func (s *structLiteral) matchesLiteral(node dst.Node) bool {
	lit, ok := node.(*dst.CompositeLit)
	if !ok {
		return false
	}
	return s.TypeName.Matches(lit.Type)
}

func (s *structLiteral) Hash(h *fingerprint.Hasher) error {
	return h.Named("struct-literal", s.TypeName, fingerprint.String(s.Field), s.Match)
}

func init() {
	unmarshalers["struct-definition"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var spec string
		if err := yaml.NodeToValueContext(ctx, node, &spec); err != nil {
			return nil, err
		}

		tn, err := typed.NewTypeName(spec)
		if err != nil {
			return nil, err
		}
		if tn.Pointer {
			return nil, fmt.Errorf("struct-definition type must not be a pointer (got %q)", spec)
		}

		return StructDefinition(tn), nil
	}
	unmarshalers["struct-literal"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var spec struct {
			Type  string
			Field string
			Match StructLiteralMatch
		}
		if err := yaml.NodeToValueContext(ctx, node, &spec); err != nil {
			return nil, err
		}

		tn, err := typed.NewTypeName(spec.Type)
		if err != nil {
			return nil, err
		}

		if spec.Field != "" {
			if spec.Match != StructLiteralMatchAny {
				return nil, fmt.Errorf("struct-literal.field is not allowed with struct-literal.match: %s", spec.Match)
			}
			return StructLiteralField(tn, spec.Field), nil
		}

		return StructLiteral(tn, spec.Match), nil
	}
}

var _ yaml.NodeUnmarshalerContext = (*StructLiteralMatch)(nil)

func (s *StructLiteralMatch) UnmarshalYAML(ctx gocontext.Context, node ast.Node) error {
	var name string
	if err := yaml.NodeToValueContext(ctx, node, &name); err != nil {
		return err
	}

	switch name {
	case "any":
		*s = StructLiteralMatchAny
	case "value-only":
		*s = StructLiteralMatchValueOnly
	case "pointer-only":
		*s = StructLiteralMatchPointerOnly
	default:
		return fmt.Errorf("invalid struct-literal.match value: %q", name)
	}

	return nil
}

func (s StructLiteralMatch) String() string {
	switch s {
	case StructLiteralMatchAny:
		return "any"
	case StructLiteralMatchValueOnly:
		return "value-only"
	case StructLiteralMatchPointerOnly:
		return "pointer-only"
	default:
		panic(fmt.Errorf("invalid StructLiteralMatch(%d)", int(s)))
	}
}

func (s StructLiteralMatch) Hash(h *fingerprint.Hasher) error {
	return h.Named("struct-literal-match", fingerprint.Int(s))
}
