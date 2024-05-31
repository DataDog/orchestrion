// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"strings"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type reference struct {
	importPath string
	name       string
}

func Reference(importPath, name string) *reference {
	return &reference{importPath, name}
}

func (r *reference) Matches(node *node.Chain) bool {
	ident, ok := node.Node.(*dst.Ident)
	if !ok {
		return false
	}

	return ident.Path == r.importPath && ident.Name == r.name
}

func (r *reference) ImpliesImported() []string {
	if r.importPath == "" {
		return nil
	}
	return []string{r.importPath}

}

func (r *reference) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Reference").Call(jen.Lit(r.importPath), jen.Lit(r.name))
}

func init() {
	unmarshalers["reference"] = func(node *yaml.Node) (Point, error) {
		var (
			importPath, name string
		)
		if err := node.Decode(&name); err != nil {
			return nil, err
		}

		if index := strings.LastIndex(name, "."); index > 0 {
			importPath, name = name[:index], name[index+1:]
		}

		return Reference(importPath, name), nil
	}
}
