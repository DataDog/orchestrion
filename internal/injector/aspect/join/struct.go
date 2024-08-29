// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type structDefinition struct {
	typeName TypeName
}

// StructDefinition matches the definition of a particular struct given its fully qualified name.
func StructDefinition(typeName TypeName) *structDefinition {
	return &structDefinition{
		typeName: typeName,
	}
}

func (s *structDefinition) ImpliesImported() []string {
	if path := s.typeName.ImportPath(); path != "" {
		return []string{path}
	}
	return nil
}

func (s *structDefinition) Matches(ctx context.AspectContext) bool {
	if s.typeName.pointer {
		// We can't ever match a pointer definition
		return false
	}

	spec, ok := ctx.Node().(*dst.TypeSpec)
	if !ok || spec.Name == nil || spec.Name.Name != s.typeName.name {
		return false
	}

	if _, ok := spec.Type.(*dst.StructType); !ok {
		return false
	}

	return ctx.ImportPath() == s.typeName.path
}

func (s *structDefinition) AsCode() jen.Code {
	return jen.Qual(pkgPath, "StructDefinition").Call(s.typeName.AsCode())
}

func (s *structDefinition) RenderHTML() string {
	return fmt.Sprintf(`<div class="flex join-point struct-definition"><span class="type">Definition of</span>%s</div>`, s.typeName.RenderHTML())
}

type structLiteral struct {
	typeName TypeName
	field    string
	match    string
}

func StructLiteral(typeName TypeName, field string, match string) *structLiteral {
	return &structLiteral{
		typeName: typeName,
		field:    field,
		match:    match,
	}
}

func (s *structLiteral) ImpliesImported() []string {
	if path := s.typeName.ImportPath(); path != "" {
		return []string{path}
	}
	return nil
}

func (s *structLiteral) Matches(ctx context.AspectContext) bool {
	if s.field == "" {
		switch s.match {
		case "pointer-only":
			// match only if the current node is equal to & and the underlying node matches
			// the struct literal we are looking for
			if expr, ok := ctx.Node().(*dst.UnaryExpr); ok && expr.Op == token.AND {
				return s.matchesLiteral(expr.X)
			}
			return false

		case "value-only":
			// do not match if the parent is equal to &
			if parent := ctx.Parent(); parent != nil {
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

	if parent := ctx.Parent(); parent == nil || !s.matchesLiteral(parent.Node()) {
		return false
	}

	key, ok := kve.Key.(*dst.Ident)
	if !ok {
		return false
	}

	return key.Name == s.field
}

func (s *structLiteral) matchesLiteral(node dst.Node) bool {
	lit, ok := node.(*dst.CompositeLit)
	if !ok {
		return false
	}
	return s.typeName.Matches(lit.Type)
}

func (s *structLiteral) AsCode() jen.Code {
	return jen.Qual(pkgPath, "StructLiteral").Call(s.typeName.AsCode(), jen.Lit(s.field), jen.Lit(s.match))
}

func (s *structLiteral) RenderHTML() string {
	var buf strings.Builder

	buf.WriteString("<div class=\"join-point struct-literal\">\n")
	buf.WriteString("  <div class=\"flex\">\n")
	buf.WriteString("    <span class=\"type\">Struct literal</span>\n")
	buf.WriteString(s.typeName.RenderHTML())
	buf.WriteString("\n  </div>\n")
	if s.field != "" {
		buf.WriteString("  <ul>\n")
		buf.WriteString("    <li class=\"flex\">\n")
		buf.WriteString("      <span class=\"type\">Including field</span>\n")
		buf.WriteString("      <code>\n")
		buf.WriteString(s.field)
		buf.WriteString("\n      </code>\n")
		buf.WriteString("    </li>\n")
		buf.WriteString("  </ul>\n")
	}
	buf.WriteString("</div>\n")

	return buf.String()
}

func init() {
	unmarshalers["struct-definition"] = func(node *yaml.Node) (Point, error) {
		var spec string
		if err := node.Decode(&spec); err != nil {
			return nil, err
		}

		tn, err := NewTypeName(spec)
		if err != nil {
			return nil, err
		}
		if tn.pointer {
			return nil, fmt.Errorf("struct-definition type must not be a pointer (got %q)", spec)
		}

		return StructDefinition(tn), nil
	}
	unmarshalers["struct-literal"] = func(node *yaml.Node) (Point, error) {
		var spec struct {
			Type  string
			Field string
			Match string
		}
		if err := node.Decode(&spec); err != nil {
			return nil, err
		}

		tn, err := NewTypeName(spec.Type)
		if err != nil {
			return nil, err
		}
		if spec.Match == "" {
			spec.Match = "any"
		}
		return StructLiteral(tn, spec.Field, spec.Match), nil
	}
}
