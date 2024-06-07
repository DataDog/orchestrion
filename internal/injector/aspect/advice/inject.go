// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"
	"errors"

	"github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type injectDeclarations struct {
	template code.Template
}

// InjectDeclarations merges all declarations in the provided source file into the current file. The package name of both
// original & injected files must match.
func InjectDeclarations(template code.Template) injectDeclarations {
	return injectDeclarations{template}
}

func (a injectDeclarations) Apply(ctx context.Context, chain *node.Chain, _ *dstutil.Cursor) (bool, error) {
	decls, err := a.template.CompileDeclarations(ctx, chain)
	if err != nil {
		return false, err
	}

	file, ok := node.Find[*dst.File](chain)
	if !ok {
		return false, errors.New("cannot inject source file: no *dst.File in context")
	}

	file.Decls = append(file.Decls, decls...)

	return true, nil
}

func (a injectDeclarations) AsCode() jen.Code {
	return jen.Qual(pkgPath, "InjectDeclarations").Call(a.template.AsCode())
}

func (a injectDeclarations) AddedImports() []string {
	return nil
}

func init() {
	unmarshalers["inject-declarations"] = func(node *yaml.Node) (Advice, error) {
		var template code.Template
		if err := node.Decode(&template); err != nil {
			return nil, err
		}

		return InjectDeclarations(template), nil
	}
}
