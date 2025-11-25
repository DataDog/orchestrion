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

type prependStatements struct {
	Template *code.Template

	order     int
	namespace string
}

// PrependStmts prepends statements to the matched *dst.BlockStmt. This action
// can only be used if the selector matches on a *dst.BlockStmt. The prepended
// statements are wrapped in a new block statement to prevent scope leakage.
func PrependStmts(template *code.Template) *prependStatements {
	return &prependStatements{
		Template: template,

		order:     DefaultOrder,
		namespace: DefaultNamespace,
	}
}

// PrependStmtsWithOrder creates a prepend-statements advice with explicit ordering
func PrependStmtsWithOrder(template *code.Template, namespace string, order int) *prependStatements {
	return &prependStatements{
		Template: template,

		order:     order,
		namespace: namespace,
	}
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
	return h.Named("prepend-statements", a.Template, fingerprint.Int(a.order), fingerprint.String(a.namespace))
}

func (a *prependStatements) AddedImports() []string {
	return a.Template.AddedImports()
}

func (a *prependStatements) Order() int {
	return a.order
}

func (a *prependStatements) Namespace() string {
	return a.namespace
}

func init() {
	unmarshalers["prepend-statements"] = func(ctx gocontext.Context, node ast.Node) (Advice, error) {
		config := struct {
			Template  code.Template `yaml:",inline"`
			Order     int           `yaml:"order,omitempty"`
			Namespace string        `yaml:"namespace,omitempty"`
		}{
			Order:     DefaultOrder,
			Namespace: DefaultNamespace,
		}

		if err := yaml.NodeToValueContext(ctx, node, &config); err != nil {
			return nil, err
		}

		if config.Namespace == "" {
			// If someone sets it to empty, reset to default.
			config.Namespace = DefaultNamespace
		}

		return &prependStatements{
			Template:  &config.Template,
			order:     config.Order,
			namespace: config.Namespace,
		}, nil
	}
}
