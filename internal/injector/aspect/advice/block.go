// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"fmt"

	"github.com/dave/dst"
	"gopkg.in/yaml.v3"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
)

type prependStatements struct {
	Template *code.Template
}

// PrependStmts prepends statements to the matched *dst.BlockStmt. This action
// can only be used if the selector matches on a *dst.BlockStmt. The prepended
// statements are wrapped in a new block statement to prevent scope leakage.
func PrependStmts(template *code.Template) *prependStatements {
	return &prependStatements{Template: template}
}

func (a *prependStatements) Apply(ctx context.AdviceContext) (bool, error) {
	block, ok := ctx.Node().(*dst.BlockStmt)
	if !ok {
		return false, fmt.Errorf("prepend-statements: expected *dst.BlockStmt, got %T", ctx.Node())
	}

	stmts, err := a.Template.CompileBlock(ctx)
	if err != nil {
		return false, fmt.Errorf("prepend-statements: %w", err)
	}

	list := make([]dst.Stmt, 1+len(block.List))
	list[0] = stmts
	copy(list[1:], block.List)
	block.List = list

	ctx.EnsureMinGoLang(a.Template.Lang)

	return true, nil
}

func (a *prependStatements) Hash(h *fingerprint.Hasher) error {
	return h.Named("prepend-statements", a.Template)
}

func (a *prependStatements) AddedImports() []string {
	return a.Template.AddedImports()
}

func init() {
	unmarshalers["prepend-statements"] = func(node *yaml.Node) (Advice, error) {
		var template *code.Template
		if err := node.Decode(&template); err != nil {
			return nil, err
		}

		return PrependStmts(template), nil
	}
}
