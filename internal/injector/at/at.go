// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package at provides implementations of the InjectionPoint interface for
// common injection points.
package at

import (
	"fmt"
	"regexp"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// InjectionPoint is the interface that abstracts selection of nodes where to
// inject code.
type InjectionPoint interface {
	// Matches determines whether the injection should be performed on the given
	// node or not.
	Matches(*dstutil.Cursor) bool

	// matchesNode is the same as Matches, except it operates on the node and its
	// parent, rather than on the *dstutil.Cursor
	matchesNode(node dst.Node, parent dst.Node) bool
}

type TypeName struct {
	// Path is the import path that provides the type, or an empty string if the
	// type is local.
	Path string
	// Name is the leaf (un-qualified) name of the type.
	Name string
	// Pointer determines whether the specified type is a pointer or not.
	Pointer bool
}

var typeNameRe = regexp.MustCompile(`\A(\*)?(?:([A-Za-z_][A-Za-z0-9_]+(?:/[A-Za-z_][A-Za-z0-9_]+)*)\.)?([A-Za-z_][A-Za-z0-9_]+)\z`)

func parseTypeName(n string) (tn TypeName, err error) {
	matches := typeNameRe.FindStringSubmatch(n)
	if matches == nil {
		err = fmt.Errorf("invalid TypeName syntax: %q", n)
		return
	}

	tn.Pointer = matches[1] == "*"
	tn.Path = matches[2]
	tn.Name = matches[3]
	return
}

func (n *TypeName) matches(node dst.Expr) bool {
	switch node := node.(type) {
	case *dst.Ident:
		return !n.Pointer && n.Path == node.Path && n.Name == node.Name

	case *dst.SelectorExpr:
		var path string
		if ident, ok := node.X.(*dst.Ident); ok && ident.Path == "" {
			path = ident.Name
		} else {
			return false
		}
		return !n.Pointer && n.Path == path && n.Name == node.Sel.Name

	case *dst.StarExpr:
		return n.Pointer && (&TypeName{Path: n.Path, Name: n.Name}).matches(node.X)

	case *dst.InterfaceType:
		// We only match the empty interface (as "any")
		if len(node.Methods.List) != 0 {
			return false
		}
		return n.Path == "" && n.Name == "any"

	default:
		return false
	}
}
