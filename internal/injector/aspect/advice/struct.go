// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/aspect/join"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type addStuctField struct {
	fieldName string
	fieldType join.TypeName
}

// AddStructField adds a new synthetic field at the tail end of a struct declaration.
func AddStructField(fieldName string, fieldType join.TypeName) *addStuctField {
	return &addStuctField{fieldName, fieldType}
}

func (a *addStuctField) Apply(_ context.Context, chain *node.Chain, _ *dstutil.Cursor) (bool, error) {
	node, ok := chain.Node.(*dst.TypeSpec)
	if !ok {
		return false, fmt.Errorf("add-struct-field advice can only be applied to *dst.TypeSpec (got %T)", chain.Node)
	}

	typeDef, ok := node.Type.(*dst.StructType)
	if !ok {
		return false, fmt.Errorf("add-struct-field advice can only be applied to struct definitions (got %T)", node.Type)
	}

	if typeDef.Fields == nil {
		typeDef.Fields = &dst.FieldList{}
	}

	typeDef.Fields.List = append(typeDef.Fields.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent(a.fieldName)},
		Type:  a.fieldType.AsNode(),
	})

	return true, nil
}

func (a *addStuctField) AsCode() jen.Code {
	return jen.Qual(pkgPath, "AddStructField").Call(jen.Lit(a.fieldName), a.fieldType.AsCode())
}

func init() {
	unmarshalers["add-struct-field"] = func(node *yaml.Node) (Advice, error) {
		var spec struct {
			Name string
			Type string
		}

		if err := node.Decode(&spec); err != nil {
			return nil, err
		}
		tn, err := join.NewTypeName(spec.Type)
		if err != nil {
			return nil, err
		}

		return AddStructField(spec.Name, tn), nil
	}
}
