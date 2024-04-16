// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package join provides implementations of the InjectionPoint interface for
// common injection points.
package join

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/datadog/orchestrion/internal/injector/code"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
)

const pkgPath = "github.com/datadog/orchestrion/internal/injector/aspect/join"

// Point is the interface that abstracts selection of nodes where to inject
// code.
type Point interface {
	code.AsCode

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

var typeNameRe = regexp.MustCompile(`\A(\*)?(?:([A-Za-z_][A-Za-z0-9_]+(?:/[A-Za-z_][A-Za-z0-9_]+)*)\.)?([A-Za-z_][A-Za-z0-9_]*)\z`)

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

// MustTypeName is the same as NewTypeName, except it panics in case of an error.
func MustTypeName(n string) (tn TypeName) {
	var err error
	if tn, err = NewTypeName(n); err != nil {
		panic(err)
	}
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

func (n *TypeName) AsNode() dst.Expr {
	ident := dst.NewIdent(n.name)
	ident.Path = n.path
	if n.pointer {
		return &dst.StarExpr{X: ident}
	}
	return ident
}

func (n *TypeName) AsCode() jen.Code {
	str := bytes.NewBuffer(make([]byte, 0, 1+len(n.path)+len(n.name)))
	if n.pointer {
		str.WriteString("*")
	}
	if n.path != "" {
		str.WriteString(n.path)
		str.WriteString(".")
	}
	str.WriteString(n.name)

	return jen.Qual(pkgPath, "MustTypeName").Call(jen.Lit(str.String()))
}
