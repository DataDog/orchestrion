// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"errors"
	"fmt"

	"github.com/dave/dst"
)

type (
	function interface {
		// Receiver returns the name of the receiver of this method. Fails if the current function is
		// not a method.
		Receiver() (string, error)
		// Name returns the name of this function, or an empty string if it is a function literal.
		Name() (string, error)
		// Argument returns the name of the argument at the given index in this function's type, returning
		// an error if the index is out of bounds.
		Argument(int) (string, error)
		// Returns returns the name of the return value at the given index in this function's type,
		// returning an error if the index is out of bounds.
		Returns(int) (string, error)
	}

	declaredFunc struct {
		Decl *dst.FuncDecl
	}

	literalFunc struct {
		Lit *dst.FuncLit
	}

	noFunc struct{}
)

var errNotMethod = errors.New("the function in this context is not a method")

func (d *dot) Function() function {
	for curr := d.node; curr != nil; curr = curr.Parent() {
		switch node := curr.Node.(type) {
		case *dst.FuncDecl:
			return &declaredFunc{node}
		case *dst.FuncLit:
			return &literalFunc{node}
		}
	}
	return &noFunc{}
}

func (f *declaredFunc) Receiver() (string, error) {
	if f.Decl.Recv == nil {
		return "", errNotMethod
	}
	return fieldAt(f.Decl.Recv, 0, "receiver")
}

func (f *declaredFunc) Name() (string, error) {
	return f.Decl.Name.Name, nil
}

func (f *declaredFunc) Argument(index int) (name string, err error) {
	name, err = argument(f.Decl.Type, index)
	return
}

func (f *declaredFunc) Returns(index int) (name string, err error) {
	name, err = returns(f.Decl.Type, index)
	return
}

func (f *literalFunc) Receiver() (string, error) {
	return "", errNotMethod
}

func (f *literalFunc) Name() (string, error) {
	return "", nil
}

func (f *literalFunc) Argument(index int) (name string, err error) {
	name, err = argument(f.Lit.Type, index)
	return
}

func (f *literalFunc) Returns(index int) (name string, err error) {
	name, err = returns(f.Lit.Type, index)
	return
}

var errNoFunction = errors.New("no function is present in this node chain")

func (f *noFunc) Receiver() (string, error) {
	return "", errNoFunction
}

func (f *noFunc) Name() (string, error) {
	return "", errNoFunction
}

func (f *noFunc) Argument(index int) (string, error) {
	return "", errNoFunction
}

func (f *noFunc) Returns(index int) (string, error) {
	return "", errNoFunction
}

func argument(ft *dst.FuncType, index int) (name string, err error) {
	name, err = fieldAt(ft.Params, index, "argument")
	return
}

func returns(ft *dst.FuncType, index int) (name string, err error) {
	name, err = fieldAt(ft.Results, index, "returns")
	return
}

func fieldAt(fields *dst.FieldList, index int, use string) (name string, err error) {
	if fields == nil || len(fields.List) == 0 {
		return "", fmt.Errorf("index out of bounds: %d (empty set)", index)
	}

	idx := 0
	anonymous := false
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			anonymous = true
			// Give a name to all items (if there are unnamed items, all items are unnamed)
			field.Names = []*dst.Ident{dst.NewIdent("_")}
		}

		for _, ident := range field.Names {
			if idx == index {
				if ident.Name == "_" {
					// Give it a referenceable name if necessary.
					ident.Name = fmt.Sprintf("__%s__%d", use, index)
				}
				name = ident.Name
				if !anonymous {
					// If the items were not anonymous, we can return immediately!
					return
				}
			}
			idx += 1
		}
	}
	return
}
