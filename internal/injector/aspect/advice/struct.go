// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	gocontext "context"
	"fmt"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type addStructField struct {
	Name     string
	TypeName typed.TypeName
}

// AddStructField adds a new synthetic field at the tail end of a struct declaration.
func AddStructField(fieldName string, fieldType typed.TypeName) *addStructField {
	return &addStructField{fieldName, fieldType}
}

func (a *addStructField) Apply(ctx context.AdviceContext) (bool, error) {
	node, ok := ctx.Node().(*dst.TypeSpec)
	if !ok {
		return false, fmt.Errorf("add-struct-field advice can only be applied to *dst.TypeSpec (got %T)", ctx.Node())
	}

	typeDef, ok := node.Type.(*dst.StructType)
	if !ok {
		return false, fmt.Errorf("add-struct-field advice can only be applied to struct definitions (got %T)", node.Type)
	}

	if typeDef.Fields == nil {
		typeDef.Fields = &dst.FieldList{}
	}

	typeDef.Fields.List = append(typeDef.Fields.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent(a.Name)},
		Type:  a.TypeName.AsNode(),
	})

	if importPath := a.TypeName.ImportPath; importPath != "" {
		// If the type name is qualified, we may need to import the package, too.
		_ = ctx.AddImport(importPath, inferPkgName(importPath))
	}

	return true, nil
}

func (a *addStructField) Hash(h *fingerprint.Hasher) error {
	return h.Named("add-struct-field", fingerprint.String(a.Name), a.TypeName)
}

func (a *addStructField) AddedImports() []string {
	if path := a.TypeName.ImportPath; path != "" {
		return []string{path}
	}
	return nil
}

func init() {
	unmarshalers["add-struct-field"] = func(ctx gocontext.Context, node ast.Node) (Advice, error) {
		var spec struct {
			Name string
			Type string
		}

		if err := yaml.NodeToValueContext(ctx, node, &spec); err != nil {
			return nil, err
		}
		tn, err := typed.NewTypeName(spec.Type)
		if err != nil {
			return nil, err
		}

		return AddStructField(spec.Name, tn), nil
	}
}
