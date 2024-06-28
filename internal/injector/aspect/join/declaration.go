// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type valueDeclaration struct {
	typeName TypeName
}

func ValueDeclaration(typeName TypeName) *valueDeclaration {
	return &valueDeclaration{typeName}
}

func (i *valueDeclaration) Matches(node *node.Chain) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}

	if _, ok := parent.Node.(*dst.GenDecl); !ok {
		return false
	}

	spec, ok := node.Node.(*dst.ValueSpec)
	if !ok {
		return false
	}

	return spec.Type == nil || i.typeName.Matches(spec.Type)
}

func (i *valueDeclaration) ImpliesImported() []string {
	if path := i.typeName.ImportPath(); path != "" {
		return []string{path}
	}
	return nil
}

func (i *valueDeclaration) AsCode() jen.Code {
	return jen.Qual(pkgPath, "ValueDeclaration").Call(i.typeName.AsCode())
}

func (i *valueDeclaration) RenderHTML() string {
	return fmt.Sprintf(`<div class="flex join-point value-declaration"><span class="type">Package-level <code style="display:inline">const</code> or <code style="display:inline">var</code></span>%s</div>`, i.typeName.RenderHTML())
}

func init() {
	unmarshalers["value-declaration"] = func(node *yaml.Node) (Point, error) {
		var typeName string
		if err := node.Decode(&typeName); err != nil {
			return nil, err
		}

		tn, err := NewTypeName(typeName)
		if err != nil {
			return nil, err
		}

		return ValueDeclaration(tn), nil
	}
}
