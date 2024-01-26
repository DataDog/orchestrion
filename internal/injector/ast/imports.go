// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ast

import (
	"context"
	"errors"
	"fmt"
	"go/token"

	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type addImport struct {
	path string
}

func AddImport(path string) *addImport {
	return &addImport{path: path}
}

func (a *addImport) Apply(ctx context.Context, _ *dstutil.Cursor) (bool, error) {
	file, ok := typed.ContextValue[*dst.File](ctx)
	if !ok {
		return false, errors.New("unable to obtain *dst.File from context")
	}

	add := &dst.ImportSpec{Path: &dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%#v", a.path)}}

	for _, spec := range file.Imports {
		if spec.Path.Value == add.Path.Value && spec.Name == nil {
			return false, nil
		}
	}

	file.Imports = append(file.Imports, add)
	var spec *dst.GenDecl
	for _, decl := range file.Decls {
		if gen, ok := decl.(*dst.GenDecl); ok && gen.Tok == token.IMPORT {
			spec = gen
			break
		}
	}
	if spec == nil {
		spec = &dst.GenDecl{Tok: token.IMPORT}
		decls := make([]dst.Decl, 1+len(file.Decls))
		decls[0] = spec
		copy(decls[1:], file.Decls)
		file.Decls = decls
	}

	spec.Specs = append(spec.Specs, add)

	if refmap, ok := typed.ContextValue[*typed.ReferenceMap](ctx); ok {
		if *refmap == nil {
			*refmap = make(typed.ReferenceMap)
		}
		(*refmap)[a.path] = typed.ImportStatement
	}

	return true, nil
}

func init() {
	unmarshalers["add-import"] = func(node *yaml.Node) (Action, error) {
		var path string
		if err := node.Decode(&path); err != nil {
			return nil, err
		}
		return AddImport(path), nil
	}
}
