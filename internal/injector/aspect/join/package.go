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

type importPath string

func ImportPath(name string) importPath {
	return importPath(name)
}

func (p importPath) ImpliesImported() []string {
	return []string{string(p)} // Technically the current package in this instance
}

func (p importPath) Matches(chain *node.Chain) bool {
	return chain.ImportPath() == string(p)
}

func (p importPath) AsCode() jen.Code {
	return jen.Qual(pkgPath, "ImportPath").Call(jen.Lit(string(p)))
}

func (p importPath) ToHTML() string {
	return fmt.Sprintf(`Import path is <a href="http://pkg.go.dev/%[1]s" target="_blank" rel="noopener"><code>%[1]s</code></a>`, p)
}

type packageName string

func PackageName(name string) packageName {
	return packageName(name)
}

func (packageName) ImpliesImported() []string {
	return nil // Can't assume anything here...
}

func (p packageName) Matches(chain *node.Chain) bool {
	file, found := node.Find[*dst.File](chain)
	return found && file.Name.Name == string(p)
}

func (p packageName) AsCode() jen.Code {
	return jen.Qual(pkgPath, "PackageName").Call(jen.Lit(string(p)))
}

func (p packageName) ToHTML() string {
	return fmt.Sprintf("Package is named <code>%s</code>", p)
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
