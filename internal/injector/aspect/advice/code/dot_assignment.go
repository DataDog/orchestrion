// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"errors"
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
)

type assignment struct {
	*placeholders
	stmt  *dst.AssignStmt
	index int
}

// Assignment returns a resolver for the closest assignment statement in the context of the template.
// It returns nil if no assignment is present in the node chain.
func (d *dot) Assignment() *assignment {
	stmt, found := node.Find[*dst.AssignStmt](d.node)
	if !found {
		return nil
	}

	idx := -1
	for curr := d.node; curr != nil; curr = curr.Parent() {
		if curr.Node == stmt {
			idx = curr.Index()
			break
		}
	}

	return &assignment{&d.placeholders, stmt, idx}
}

type assignmentLhs struct {
	*placeholders
	expr dst.Expr
}

func (a *assignment) LHS() (*assignmentLhs, error) {
	var lhs dst.Expr
	if len(a.stmt.Lhs) == 1 {
		lhs = a.stmt.Lhs[0]
	} else if a.index >= 0 {
		if len(a.stmt.Lhs) >= a.index {
			return nil, fmt.Errorf("index is out of bounds (%d >= %d)", a.index, len(a.stmt.Lhs))
		}
		lhs = a.stmt.Lhs[a.index]
	} else {
		return nil, errors.New("multiple LHS expressions are present, but no index is available")
	}

	return &assignmentLhs{a.placeholders, lhs}, nil
}

func (lhs *assignmentLhs) String() string {
	return lhs.placeholders.forNode(lhs.expr, false)
}
