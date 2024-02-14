// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"context"
	"fmt"
	"go/token"
	"strconv"

	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// Resolve resolves the provided `dst.Node`s
func Resolve[N dst.Node, S ~[]N](ctx context.Context, nodes S) S {
	for i, node := range nodes {
		nodes[i] = dstutil.Apply(
			node,
			func(cursor *dstutil.Cursor) bool {
				chain, ok := selectorChain(cursor.Node())
				if !ok {
					// This is not a relevant selector chain. Ignore it.
					return true
				}

				switch chain[0] {
				case "FuncDecl":
					decl, ok := typed.ContextValue[*dst.FuncDecl](ctx)
					if !ok {
						panic(fmt.Errorf("failed to obtain capture for FuncDecl"))
					}
					switch chain[1] {
					case "Args":
						if len(chain) != 3 {
							return true
						}
						index, err := strconv.Atoi(chain[2])
						if err != nil {
							panic(fmt.Errorf("invalid argument index %q: %w", chain[2], err))
						}
						cursor.Replace(dst.Clone(decl.Type.Params.List[index].Names[0]))
					case "Name":
						if len(chain) != 2 {
							return true
						}
						cursor.Replace(&dst.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%#v", decl.Name.Name)})
					default:
						panic(fmt.Errorf("unknown FuncDecl capture: %q", chain[1]))
					}
				default:
					panic(fmt.Errorf("unknown capture: %q", chain[0]))
				}

				// We've replaced this node, no need to look into children anymore
				return false
			},
			nil,
		).(N)
	}

	return nodes
}

// selectorChain identifies chains of selector expressions rooted on `_` and
// index expressions on these, which represent placeholders for AST captures.
// The leading `_` is not included in the returned chain.
func selectorChain(node dst.Node) ([]string, bool) {
	if idx, ok := node.(*dst.IndexExpr); ok {
		parent, ok := selectorChain(idx.X)
		if !ok {
			return nil, false
		}
		index, ok := idx.Index.(*dst.BasicLit)
		if !ok || index.Kind != token.INT {
			return nil, false
		}
		return append(parent, index.Value), true
	}

	sel, ok := node.(*dst.SelectorExpr)
	if !ok || sel.Sel.Path != "" {
		return nil, false
	}

	if id, ok := sel.X.(*dst.Ident); ok {
		if id.Path != "" || id.Name != "_" {
			return nil, false
		}
		chain := make([]string, 1, 3)
		chain[0] = sel.Sel.Name
		return chain, true
	} else {
		parent, ok := selectorChain(sel.X)
		if !ok {
			return nil, false
		}
		return append(parent, sel.Sel.Name), true
	}
}
