// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"errors"
	"go/token"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
)

type assignment struct {
	Stmt *dst.AssignStmt
}

// Assignment returns a resolver for the closest assignment statement in the context of the template.
// It returns nil if no assignment is present in the node chain.
func (d *dot) Assignment() *assignment {
	stmt, found := node.Find[*dst.AssignStmt](d.node)
	if !found {
		return nil
	}
	return &assignment{stmt}
}

func (a *assignment) Variable() (string, error) {
	if len(a.Stmt.Lhs) != 1 {
		return "", errors.ErrUnsupported
	}

	ident, ok := a.Stmt.Lhs[0].(*dst.Ident)
	if !ok {
		return "", errors.ErrUnsupported
	}

	if ident.Name == "_" {
		// Give it a referenceable name
		ident.Name = "__assigned"
		// If this was a plain assignment, upgrade it to a definition
		if a.Stmt.Tok == token.ASSIGN {
			a.Stmt.Tok = token.DEFINE
		}
	}
	return ident.Name, nil
}
