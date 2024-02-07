// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// dot provides the `.` value to code templates, and is used to access various bits of
// information from the template's rendering context.
type (
	placeholders struct {
		singletons map[dst.Expr]string
		byName     map[string]dst.Expr
	}

	dot struct {
		node         *node.Chain // The node in context of which the template is rendered
		placeholders             // Placeholders used by the template
		expr         dst.Expr    // The expression to be used in the template (as {{.}}), if any
	}
)

func (d *dot) String() string {
	if d.expr != nil {
		return d.placeholders.forNode(d.expr, true)
	}
	return fmt.Sprintf("/* %s */", d.node.String())
}

// forNode obtains the placeholder syntax to use for referencing the given node. If singleton is
// true, this returns the same placeholder for each invocation with the same node argument.
// Otherwise, this returns a new placeholder for each invocation, guaranteeing that different AST
// nodes are produced (it's an error to have the same AST node multiple times in the output AST).
func (p *placeholders) forNode(node dst.Expr, singleton bool) string {
	if singleton {
		if name, found := p.singletons[node]; found {
			return name
		}
		if p.singletons == nil {
			// Will be filled in later once we have determined the name
			p.singletons = make(map[dst.Expr]string, 1)
		}
	}

	name := fmt.Sprintf("__PLACEHOLDER_%d__", len(p.byName))
	if p.byName == nil {
		p.byName = make(map[string]dst.Expr, 1)
	}
	if singleton {
		p.byName[name] = node
		p.singletons[node] = name
	} else {
		p.byName[name] = dst.Clone(node).(dst.Expr)
	}

	return fmt.Sprintf("_.%s", name)
}

// replaceAllIn replaces all placeholders found in the given AST with the actual dst.Expr value.
func (p *placeholders) replaceAllIn(ast dst.Node) dst.Node {
	if len(p.byName) == 0 {
		return ast
	}

	return dstutil.Apply(
		ast,
		func(csor *dstutil.Cursor) bool {
			selectorExpr, ok := csor.Node().(*dst.SelectorExpr)
			if !ok {
				return true
			}

			repl, found := p.byName[selectorExpr.Sel.Name]
			if !found {
				return true
			}

			if ident, ok := selectorExpr.X.(*dst.Ident); !ok || ident.Name != "_" {
				return true
			}

			csor.Replace(repl)

			return false
		},
		nil,
	)
}
