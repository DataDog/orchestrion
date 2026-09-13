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

type replaceStatement struct {
	Template *code.Template
}

func (replacement *replaceStatement) Apply(ctx context.AdviceContext) (bool, error) {
	if _, ok := ctx.Node().(dst.Stmt); !ok {
		return false, fmt.Errorf("replace-statement: expected dst.Stmt, got %T", ctx.Node())
	}
	block, err := replacement.Template.CompileBlock(ctx)
	if err != nil {
		return false, fmt.Errorf("replace-statement: %w", err)
	}
	if len(block.List) != 1 {
		return false, fmt.Errorf("replace-statement: template produced %d statements, want 1", len(block.List))
	}
	ctx.ReplaceNode(block.List[0])
	ctx.EnsureMinGoLang(replacement.Template.Lang)
	return true, nil
}

func (replacement *replaceStatement) AddedImports() []string {
	return replacement.Template.AddedImports()
}

func (replacement *replaceStatement) Hash(h *fingerprint.Hasher) error {
	return h.Named("replace-statement", replacement.Template)
}

func init() {
	unmarshalers["replace-statement"] = func(ctx gocontext.Context, node ast.Node) (Advice, error) {
		var template code.Template
		if err := yaml.NodeToValueContext(ctx, node, &template); err != nil {
			return nil, err
		}
		return &replaceStatement{Template: &template}, nil
	}
}
