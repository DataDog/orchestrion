// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type appendArgs struct {
	templates []code.Template
}

func AppendArgs(templates ...code.Template) *appendArgs {
	return &appendArgs{templates}
}

func (a *appendArgs) Apply(ctx context.Context, chain *node.Chain, csor *dstutil.Cursor) (bool, error) {
	call, ok := chain.Node.(*dst.CallExpr)
	if !ok {
		return false, fmt.Errorf("expected a *dst.CallExpr, received %T", chain.Node)
	}

	newArgs := make([]dst.Expr, len(a.templates))
	var err error
	for i, t := range a.templates {
		newArgs[i], err = t.CompileExpression(ctx, chain)
		if err != nil {
			return false, err
		}
	}

	call.Args = append(call.Args, newArgs...)

	return true, nil
}

func (a *appendArgs) AsCode() jen.Code {
	return jen.Qual(pkgPath, "AppendArgs").CallFunc(func(group *jen.Group) {
		for _, t := range a.templates {
			group.Line().Add(t.AsCode())
		}
		group.Empty().Line()
	})
}

func init() {
	unmarshalers["append-args"] = func(node *yaml.Node) (Advice, error) {
		var templates []code.Template
		if err := node.Decode(&templates); err != nil {
			return nil, err
		}
		return AppendArgs(templates...), nil
	}
}
