// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package advice provides implementations of the injector.Action interface for
// common AST changes.
package advice

import (
	"context"
	"errors"
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type addBlankImport string

func AddBlankImport(path string) addBlankImport {
	return addBlankImport(path)
}

func (a addBlankImport) Apply(ctx context.Context, chain *node.Chain, _ *dstutil.Cursor) (bool, error) {
	file, hasFile := node.Find[*dst.File](chain)
	if !hasFile {
		return false, errors.New("cannot add import: no *dst.File found in node.Chain")
	}

	refMap, hasMap := typed.ContextValue[*typed.ReferenceMap](ctx)
	if !hasMap {
		return false, errors.New("cannot add import: no *typed.ReferenceMap found in context")
	}

	refMap.AddImport(file, string(a))
	return true, nil
}

func (a addBlankImport) AsCode() jen.Code {
	return jen.Qual(pkgPath, "AddBlankImport").Call(jen.Lit(string(a)))
}

func (a addBlankImport) AddedImports() []string {
	return []string{string(a)}
}

func (a addBlankImport) RenderHTML() string {
	return fmt.Sprintf(`<span class="advice add-blank-import"><span class="type">Add blank import of </span>{{<godoc %q>}}</span>`, string(a))
}

func init() {
	unmarshalers["add-blank-import"] = func(node *yaml.Node) (Advice, error) {
		var path string
		if err := node.Decode(&path); err != nil {
			return nil, err
		}
		return AddBlankImport(path), nil
	}
}
