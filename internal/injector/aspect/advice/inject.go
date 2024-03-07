// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"

	"github.com/datadog/orchestrion/internal/injector/basiclit"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type injectSourceFile []byte

// InjectSourceFile merges all declarations in the provided source file into the current file. The package name of both
// original & injected files must match.
func InjectSourceFile(text string) injectSourceFile {
	return injectSourceFile(text)
}

func (a injectSourceFile) Apply(ctx context.Context, chain *node.Chain, _ *dstutil.Cursor) (bool, error) {
	dec, found := typed.ContextValue[*decorator.Decorator](ctx)
	if !found {
		return false, errors.New("cannot inject source file: no *decorator.Decorator in context")
	}

	file, ok := node.Find[*dst.File](chain)
	if !ok {
		return false, errors.New("cannot inject source file: no *dst.File in context")
	}

	newFile, err := dec.ParseFile("<generated>", []byte(a), parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("injecting new source file: %w", err)
	}

	if file.Name.Name != newFile.Name.Name {
		return false, fmt.Errorf("cannot inject source file: package names do not match (%s != %s)", file.Name.Name, newFile.Name.Name)
	}

	refMap, _ := typed.ContextValue[*typed.ReferenceMap](ctx)
	if refMap == nil {
		return false, fmt.Errorf("cannot inject source file: no *typed.ReferenceMap in context")
	}

	for _, decl := range newFile.Decls {
		file.Decls, err = mergeDeclaration(file, refMap, file.Decls, decl)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func mergeDeclaration(file *dst.File, refs *typed.ReferenceMap, decls []dst.Decl, newDecl dst.Decl) ([]dst.Decl, error) {
	if gen, ok := newDecl.(*dst.GenDecl); ok && gen.Tok == token.IMPORT {
		for _, spec := range gen.Specs {
			if imp, ok := spec.(*dst.ImportSpec); ok {
				importPath, err := basiclit.String(imp.Path)
				if err != nil {
					return decls, err
				}
				refs.AddImport(file, importPath)
			}
		}
		return decls, nil
	}
	return append(decls, newDecl), nil
}

func (a injectSourceFile) AsCode() jen.Code {
	return jen.Qual(pkgPath, "InjectSourceFile").Call(jen.Lit(string(a)))
}

func (a injectSourceFile) AddedImports() []string {
	return nil
}

func (a injectSourceFile) ToHTML() string {
	return fmt.Sprintf("Inject new source file containing:\n\n```go\n%s\n```\n", string(a))
}

func init() {
	unmarshalers["inject-source-file"] = func(node *yaml.Node) (Advice, error) {
		var text string
		if err := node.Decode(&text); err != nil {
			return nil, err
		}

		return InjectSourceFile(text), nil
	}
}
