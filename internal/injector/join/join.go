// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package join provides implementations of the InjectionPoint interface for
// common injection points.
package join

import (
	"fmt"
	"regexp"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
)

// Point is the interface that abstracts selection of nodes where to inject
// code.
type Point interface {
	// Matches determines whether the injection should be performed on the given
	// node or not. The node's ancestry is also provided to allow Point to make
	// decisions based on parent nodes.
	Matches(node *node.Chain) bool
}

type TypeName struct {
	// path is the import path that provides the type, or an empty string if the
	// type is local.
	path string
	// name is the leaf (un-qualified) name of the type.
	name string
	// pointer determines whether the specified type is a pointer or not.
	pointer bool
}

var typeNameRe = regexp.MustCompile(`\A(\*)?(?:([A-Za-z_][A-Za-z0-9_]+(?:/[A-Za-z_][A-Za-z0-9_]+)*)\.)?([A-Za-z_][A-Za-z0-9_]+)\z`)

func NewTypeName(n string) (tn TypeName, err error) {
	matches := typeNameRe.FindStringSubmatch(n)
	if matches == nil {
		err = fmt.Errorf("invalid TypeName syntax: %q", n)
		return
	}

	tn.pointer = matches[1] == "*"
	tn.path = matches[2]
	tn.name = matches[3]
	return
}

func (n *TypeName) Matches(node dst.Expr) bool {
	switch node := node.(type) {
	case *dst.Ident:
		return !n.pointer && n.path == node.Path && n.name == node.Name

	case *dst.SelectorExpr:
		var path string
		if ident, ok := node.X.(*dst.Ident); ok && ident.Path == "" {
			path = ident.Name
		} else {
			return false
		}
		return !n.pointer && n.path == path && n.name == node.Sel.Name

	case *dst.StarExpr:
		return n.pointer && (&TypeName{path: n.path, name: n.name}).Matches(node.X)

	case *dst.InterfaceType:
		// We only match the empty interface (as "any")
		if len(node.Methods.List) != 0 {
			return false
		}
		return n.path == "" && n.name == "any"

	default:
		return false
	}
}
