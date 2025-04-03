// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"errors"
	"fmt"
	"go/types"

	"github.com/dave/dst"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/join"
)

type (
	function interface {
		// Receiver returns the name of the receiver of this method. Fails if the current function is
		// not a method.
		Receiver() (string, error)
		// Name returns the name of this function, or an empty string if it is a function literal.
		Name() (string, error)

		// Argument returns the name of the argument at the given index in this function's type,
		// returningan error if the index is out of bounds.
		Argument(int) (string, error)
		// ArgumentOfType returns the name of the first argument in this function that has the provided
		// type, or an empty string if none is found.
		ArgumentOfType(string) (string, error)

		// Result returns the name of the return value at the given index in this function's type,
		// returning an error if the index is out of bounds.
		Result(int) (string, error)
		// ResultOfType returns the name of the first return value in this function that has the
		// provided type, or a empty string if none is found.
		ResultOfType(string) (string, error)
		// ResultImplements returns the name of the first return value in this function that implements
		// the provided interface type, or an empty string if none is found.
		ResultImplements(string) (string, error)
		// LastResultImplements returns the name of the last return value in this function that implements
		// the provided interface type, or an empty string if none is found.
		LastResultImplements(string) (string, error)
	}

	declaredFunc struct {
		signature
		Decl *dst.FuncDecl
	}

	literalFunc struct {
		signature
		Lit *dst.FuncLit
	}

	noFunc struct{}
)

var (
	errNoFunction = errors.New("no function is present in this node chain")
	errNotMethod  = errors.New("the function in this context is not a method")
)

func (d *dot) Function() function {
	for curr := d.context.Chain(); curr != nil; curr = curr.Parent() {
		switch node := curr.Node().(type) {
		case *dst.FuncDecl:
			return &declaredFunc{signature{d.context, node.Type}, node}
		case *dst.FuncLit:
			return &literalFunc{signature{d.context, node.Type}, node}
		}
	}
	return noFunc{}
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

func (*literalFunc) Receiver() (string, error) {
	return "", errNotMethod
}

func (*literalFunc) Name() (string, error) {
	return "", nil
}

func (noFunc) Receiver() (string, error) {
	return "", errNoFunction
}

func (noFunc) Name() (string, error) {
	return "", errNoFunction
}

func (noFunc) Argument(int) (string, error) {
	return "", errNoFunction
}

func (noFunc) ArgumentOfType(string) (string, error) {
	return "", errNoFunction
}

func (noFunc) Result(int) (string, error) {
	return "", errNoFunction
}

func (noFunc) ResultOfType(string) (string, error) {
	return "", errNoFunction
}

func (noFunc) ResultImplements(string) (string, error) {
	return "", errNoFunction
}

func (noFunc) LastResultImplements(string) (string, error) {
	return "", errNoFunction
}

type signature struct {
	context context.AdviceContext
	*dst.FuncType
}

func (s signature) Argument(index int) (string, error) {
	return fieldAt(s.Params, index, "argument")
}

func (s signature) ArgumentOfType(name string) (string, error) {
	return fieldOfType(s.Params, name, "argument")
}

func (s signature) Result(index int) (name string, err error) {
	return fieldAt(s.Results, index, "result")
}

func (s signature) ResultOfType(name string) (string, error) {
	return fieldOfType(s.Results, name, "result")
}

func (s signature) ResultImplements(name string) (string, error) {
	// Return blank if there are no results.
	if s.Results == nil {
		return "", nil
	}

	// Parse the interface type name
	tn, err := join.NewTypeName(name)
	if err != nil {
		return "", fmt.Errorf("cannot parse interface type name: %v", err)
	}
	// Parse the interface type.
	interfaceExpr := tn.AsNode()
	iface, err := resolveInterfaceType(s.context, interfaceExpr)
	if err != nil {
		return "", fmt.Errorf("cannot resolve type for interface %s: %v", name, err)
	}

	// Check each result.
	index := 0
	for _, field := range s.Results.List {
		if exprImplements(s.context, field.Type, iface) {
			return fieldAt(s.Results, index, "result")
		}

		switch count := len(field.Names); count {
		case 0, 1:
			index++
		default:
			index += count
		}
	}

	// Not found.
	return "", nil
}

func (s signature) LastResultImplements(name string) (string, error) {
	// Return blank if there are no results.
	if s.Results == nil {
		return "", nil
	}

	// Parse the interface type name
	tn, err := join.NewTypeName(name)
	if err != nil {
		return "", fmt.Errorf("cannot parse interface type name: %v", err)
	}

	// Parse the interface type.
	interfaceExpr := tn.AsNode()
	iface, err := resolveInterfaceType(s.context, interfaceExpr)
	if err != nil {
		return "", fmt.Errorf("cannot resolve type for interface %s: %v", name, err)
	}

	// Check each result in reverse order.
	index := 0
	lastIndex := -1
	lastField := -1

	for i, field := range s.Results.List {
		if exprImplements(s.context, field.Type, iface) {
			lastIndex = index
			lastField = i
		}

		switch count := len(field.Names); count {
		case 0, 1:
			index++
		default:
			index += count
		}
	}

	if lastField >= 0 {
		return fieldAt(s.Results, lastIndex, "result")
	}

	// Not found
	return "", nil
}

func fieldAt(fields *dst.FieldList, index int, use string) (string, error) {
	if fields == nil {
		return "", fmt.Errorf("index out of bounds: %d (empty set)", index)
	}

	idx := 0
	anonymous := false
	name := ""
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
					return name, nil
				}
			}
			idx++
		}
	}

	if idx < index {
		return "", fmt.Errorf("index out of bounds: %d (only %d items)", index, idx+1)
	}

	return name, nil
}

func fieldOfType(fields *dst.FieldList, typeName string, use string) (string, error) {
	tn, err := join.NewTypeName(typeName)
	if err != nil {
		return "", err
	}

	if fields == nil {
		// No fields, no match!
		return "", nil
	}

	index := 0
	for _, field := range fields.List {
		if tn.Matches(field.Type) {
			return fieldAt(fields, index, use)
		}
		switch count := len(field.Names); count {
		case 0, 1:
			index++
		default:
			index += count
		}
	}

	// Not found!
	return "", nil
}

// resolveInterfaceType gets the underlying interface type from a type expression.
func resolveInterfaceType(ctx context.AdviceContext, expr dst.Expr) (*types.Interface, error) {
	// Get the interface type from the context.
	interfaceType := ctx.ResolveType(expr)
	if interfaceType == nil {
		return nil, fmt.Errorf("cannot resolve type for interface expression")
	}

	// Extract the underlying interface
	switch t := interfaceType.(type) {
	case *types.Interface:
		return t, nil
	case *types.Named:
		if underlying, ok := t.Underlying().(*types.Interface); ok {
			return underlying, nil
		}
		return nil, fmt.Errorf("type is not an interface")
	default:
		return nil, fmt.Errorf("type is not an interface")
	}
}

// exprImplements checks if an expression's type implements an interface.
func exprImplements(ctx context.AdviceContext, expr dst.Expr, iface *types.Interface) bool {
	actualType := ctx.ResolveType(expr)
	if actualType == nil {
		return false
	}

	return typeImplements(actualType, iface)
}

// typeImplements checks if a type implements an interface (including pointer receivers).
func typeImplements(t types.Type, iface *types.Interface) bool {
	if t == nil || iface == nil {
		return false
	}

	// Direct implementation check
	if types.Implements(t, iface) {
		return true
	}

	// Check pointer type implementation for named types.
	if named, ok := t.(*types.Named); ok {
		ptrType := types.NewPointer(named)
		if types.Implements(ptrType, iface) {
			return true
		}
	}

	return false
}
