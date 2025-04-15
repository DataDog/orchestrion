// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"errors"
	"fmt"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
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
		// ResultThatImplements returns the name of the first return value in this function that implements
		// the provided interface type, or an empty string if none is found.
		ResultThatImplements(string) (string, error)
		// LastResultThatImplements returns the name of the last return value
		// (may not be the last item in the function's return list) in this function that implements
		// the provided interface type, or an empty string if none is found.
		LastResultThatImplements(string) (string, error)
		// FinalResultImplements returns whether the final (very last item that is returned) result implements the provided interface type.
		FinalResultImplements(string) (bool, error)
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

func (noFunc) ResultThatImplements(string) (string, error) {
	return "", errNoFunction
}

func (noFunc) LastResultThatImplements(string) (string, error) {
	return "", errNoFunction
}

func (noFunc) FinalResultImplements(string) (bool, error) {
	return false, errNoFunction
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

func (s signature) ResultThatImplements(name string) (string, error) {
	// Return blank if there are no results.
	if s.Results == nil {
		return "", nil
	}

	// Optimization: First, check for an exact match using the helper.
	if index, found := typed.FindMatchingTypeName(s.Results, name); found {
		return fieldAt(s.Results, index, "result")
	} // If not found, fall through to type resolution.

	// Resolve the interface type.
	iface, err := typed.ResolveInterfaceTypeByName(name)
	if err != nil {
		return "", fmt.Errorf("resolving interface type %q: %w", name, err)
	}

	// Check each result.
	index := 0
	for _, field := range s.Results.List {
		if typed.ExprImplements(s.context, field.Type, iface) {
			return fieldAt(s.Results, index, "result")
		}

		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		index += count
	}

	// Not found.
	return "", nil
}

func (s signature) LastResultThatImplements(name string) (string, error) {
	// Return blank if there are no results.
	if s.Results == nil {
		return "", nil
	}

	// Optimization: First, check for an exact match using TypeName parsing, finding the last one.
	lastMatchIndex := -1
	if tn, err := typed.NewTypeName(name); err == nil {
		currentIndex := 0
		for _, field := range s.Results.List {
			if tn.Matches(field.Type) {
				lastMatchIndex = currentIndex // Update last found index
			}
			// Increment index by the number of names in the field (or 1 if unnamed).
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			currentIndex += count
		}
	}
	// If we found a match via TypeName, return it.
	if lastMatchIndex != -1 {
		return fieldAt(s.Results, lastMatchIndex, "result")
	} // If parsing failed or no match, fall through to type resolution.

	// Resolve the interface type.
	iface, err := typed.ResolveInterfaceTypeByName(name)
	if err != nil {
		// Propagate error if interface resolution fails
		return "", fmt.Errorf("resolving interface type %q: %w", name, err)
	}

	// Fallback: Check using ExprImplements, iterating backward.
	// Need field indices map again for this path.
	fieldIndices := make(map[*dst.Field]int)
	index := 0
	for _, field := range s.Results.List {
		fieldIndices[field] = index
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		index += count
	}

	for i := len(s.Results.List) - 1; i >= 0; i-- {
		field := s.Results.List[i]
		if typed.ExprImplements(s.context, field.Type, iface) {
			// Found a match, return the corresponding field.
			return fieldAt(s.Results, fieldIndices[field], "result")
		}
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
			// Give a name to all items (if there are unnamed items, all items are unnamed).
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
	tn, err := typed.NewTypeName(typeName)
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

		count := len(field.Names)
		if count == 0 { // If the field is not named it's as if there is one.
			count = 1
		}
		index += count
	}

	// Not found!
	return "", nil
}

// FinalResultImplements returns whether the final result implements the provided interface type.
func (s signature) FinalResultImplements(interfaceName string) (bool, error) {
	if s.Results == nil || len(s.Results.List) == 0 {
		return false, nil
	}

	lastField := s.Results.List[len(s.Results.List)-1]

	// Optimization: First, check for an exact match using TypeName parsing.
	// Note: Not using FindMatchingTypeName as we only need to check the last field.
	if tn, err := typed.NewTypeName(interfaceName); err == nil {
		if tn.Matches(lastField.Type) {
			return true, nil
		}
	} // If parsing failed or no match, fall through to type resolution.

	iface, err := typed.ResolveInterfaceTypeByName(interfaceName)
	if err != nil {
		return false, fmt.Errorf("resolving interface type %q: %w", interfaceName, err)
	}

	// Check if the last field type implements the interface.
	return typed.ExprImplements(s.context, lastField.Type, iface), nil
}
