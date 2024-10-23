// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type importPath string

func ImportPath(name string) importPath {
	return importPath(name)
}

func (p importPath) ImpliesImported() []string {
	return []string{string(p)} // Technically the current package in this instance
}

func (p importPath) Matches(ctx context.AspectContext) bool {
	return ctx.ImportPath() == string(p)
}

func (p importPath) AsCode() jen.Code {
	return jen.Qual(pkgPath, "ImportPath").Call(jen.Lit(string(p)))
}

type packageName string

func PackageName(name string) packageName {
	return packageName(name)
}

func (packageName) ImpliesImported() []string {
	return nil // Can't assume anything here...
}

func (p packageName) Matches(ctx context.AspectContext) bool {
	return ctx.Package() == string(p)
}

func (p packageName) AsCode() jen.Code {
	return jen.Qual(pkgPath, "PackageName").Call(jen.Lit(string(p)))
}

func init() {
	unmarshalers["import-path"] = func(node *yaml.Node) (Point, error) {
		var name string
		if err := node.Decode(&name); err != nil {
			return nil, err
		}
		return ImportPath(name), nil
	}

	unmarshalers["package-name"] = func(node *yaml.Node) (Point, error) {
		var name string
		if err := node.Decode(&name); err != nil {
			return nil, err
		}
		return PackageName(name), nil
	}
}
