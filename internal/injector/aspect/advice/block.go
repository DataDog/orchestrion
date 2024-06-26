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

type prependStatements struct {
	template code.Template
}

// PrependStmts prepends statements to the matched *dst.BlockStmt. This action
// can only be used if the selector matches on a *dst.BlockStmt. The prepended
// statements are wrapped in a new block statement to prevent scope leakage.
func PrependStmts(template code.Template) *prependStatements {
	return &prependStatements{template: template}
}

func (a *prependStatements) Apply(ctx context.Context, node *node.Chain, csor *dstutil.Cursor) (bool, error) {
	block, ok := node.Node.(*dst.BlockStmt)
	if !ok {
		return false, fmt.Errorf("expected *dst.BlockStmt, got %T", node.Node)
	}

	stmts, err := a.template.CompileBlock(ctx, node)
	if err != nil {
		return false, err
	}

	list := make([]dst.Stmt, 1+len(block.List))
	list[0] = stmts
	copy(list[1:], block.List)
	block.List = list

	return true, nil
}

func (a *prependStatements) AsCode() jen.Code {
	return jen.Qual(pkgPath, "PrependStmts").Call(a.template.AsCode())
}

func (a *prependStatements) AddedImports() []string {
	return a.template.AddedImports()
}

func (a *prependStatements) RenderHTML() string {
	return fmt.Sprintf(`<div class="advice prepend-statements"><div class="type">Prepend statements produced by the following template:</div>%s</div>`, a.template.RenderHTML())
}

func init() {
	unmarshalers["prepend-statements"] = func(node *yaml.Node) (Advice, error) {
		var template code.Template
		if err := node.Decode(&template); err != nil {
			return nil, err
		}

		return PrependStmts(template), nil
	}
}
