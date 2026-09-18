// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	gocontext "context"
	"fmt"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type insertStatementsAfter struct {
	Template *code.Template
}

func (insertion *insertStatementsAfter) Apply(ctx context.AdviceContext) (bool, error) {
	if _, ok := ctx.Node().(dst.Stmt); !ok {
		return false, fmt.Errorf("insert-statements-after: expected dst.Stmt, got %T", ctx.Node())
	}
	if !ctx.CanInsertAfter() {
		return false, fmt.Errorf("insert-statements-after: statement is not contained in a list")
	}
	block, err := insertion.Template.CompileBlock(ctx)
	if err != nil {
		return false, fmt.Errorf("insert-statements-after: %w", err)
	}
	for index := len(block.List) - 1; index >= 0; index-- {
		ctx.InsertAfter(block.List[index])
	}
	ctx.EnsureMinGoLang(insertion.Template.Lang)
	return len(block.List) > 0, nil
}

func (insertion *insertStatementsAfter) AddedImports() []string {
	return insertion.Template.AddedImports()
}

func (insertion *insertStatementsAfter) Hash(h *fingerprint.Hasher) error {
	return h.Named("insert-statements-after", insertion.Template)
}

func init() {
	unmarshalers["insert-statements-after"] = func(ctx gocontext.Context, node ast.Node) (Advice, error) {
		var template code.Template
		if err := yaml.NodeToValueContext(ctx, node, &template); err != nil {
			return nil, err
		}
		return &insertStatementsAfter{Template: &template}, nil
	}
}
