// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"fmt"
	"regexp"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type declarationOf struct {
	ImportPath string
	Name       string
}

// DeclarationOf matches the (top-level) declaration of the specified symbol.
func DeclarationOf(importPath string, name string) *declarationOf {
	return &declarationOf{ImportPath: importPath, Name: name}
}

func (i *declarationOf) Matches(ctx context.AspectContext) bool {
	if ctx.ImportPath() != i.ImportPath {
		return false
	}

	switch node := ctx.Node().(type) {
	case *dst.FuncDecl:
		return node.Name != nil && node.Name.Name == i.Name
	case *dst.ValueSpec:
		if parent := ctx.Chain().Parent(); parent == nil {
			// No parent, this is almost certainly a syntax error...
			return false
		} else if _, isGenDecl := parent.Node().(*dst.GenDecl); !isGenDecl {
			// Parent isn't a GenDecl, so this is not a top-level declaration.
			return false
		}
		for _, name := range node.Names {
			if name.Name == i.Name {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (i *declarationOf) ImpliesImported() []string {
	return []string{i.ImportPath}
}

func (i *declarationOf) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	return ctx.PackageImports(i.ImportPath)
}

func (i *declarationOf) FileMayMatch(ctx *may.FileContext) may.MatchType {
	return ctx.FileContains(i.Name)
}

func (i *declarationOf) Hash(h *fingerprint.Hasher) error {
	return h.Named("declaration-of", fingerprint.String(i.ImportPath), fingerprint.String(i.Name))
}

type valueDeclaration struct {
	TypeName TypeName
}

func ValueDeclaration(typeName TypeName) *valueDeclaration {
	return &valueDeclaration{typeName}
}

func (i *valueDeclaration) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	return ctx.PackageImports(i.TypeName.ImportPath())
}

func (*valueDeclaration) FileMayMatch(_ *may.FileContext) may.MatchType {
	return may.Unknown
}

func (i *valueDeclaration) Matches(ctx context.AspectContext) bool {
	parent := ctx.Chain().Parent()
	if parent == nil {
		return false
	}

	if _, ok := parent.Node().(*dst.GenDecl); !ok {
		return false
	}

	spec, ok := ctx.Node().(*dst.ValueSpec)
	if !ok {
		return false
	}

	return spec.Type == nil || i.TypeName.Matches(spec.Type)
}

func (i *valueDeclaration) ImpliesImported() []string {
	if path := i.TypeName.ImportPath(); path != "" {
		return []string{path}
	}
	return nil
}

func (i *valueDeclaration) Hash(h *fingerprint.Hasher) error {
	return h.Named("value-declaration", i.TypeName)
}

// See: https://regex101.com/r/OXDfJ1/1
var symbolNamePattern = regexp.MustCompile(`\A(.+)\.([\p{L}_][\p{L}_\p{Nd}]*)\z`)

func init() {
	unmarshalers["declaration-of"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var symbol string
		if err := yaml.NodeToValueContext(ctx, node, &symbol); err != nil {
			return nil, err
		}

		matches := symbolNamePattern.FindStringSubmatch(symbol)
		if matches == nil {
			return nil, fmt.Errorf("invalid symbol name %q", symbol)
		}

		return DeclarationOf(matches[1], matches[2]), nil
	}

	unmarshalers["value-declaration"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var typeName string
		if err := yaml.NodeToValueContext(ctx, node, &typeName); err != nil {
			return nil, err
		}

		tn, err := NewTypeName(typeName)
		if err != nil {
			return nil, err
		}

		return ValueDeclaration(tn), nil
	}
}
