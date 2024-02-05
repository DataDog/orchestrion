// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/join"
	"github.com/dave/dst"
)

// FindArgument looks in the surrouding context for a function parameter that matches the given
// type name, and returns its name. If no such parameter exists, an empty string is returned.
func (d *dot) FindArgument(typename string) (string, error) {
	tn, err := join.NewTypeName(typename)
	if err != nil {
		return "", err
	}

	for curr := d.node; curr != nil; curr = curr.Parent() {
		var funcType *dst.FuncType
		switch node := curr.Node.(type) {
		case *dst.FuncDecl:
			funcType = node.Type
		case *dst.FuncLit:
			funcType = node.Type
		case *dst.FuncType:
			funcType = node
		default:
			continue
		}

		for idx, field := range funcType.Params.List {
			if tn.Matches(field.Type) {
				if len(field.Names) == 0 {
					field.Names = []*dst.Ident{dst.NewIdent(fmt.Sprintf("_arg_%d", idx))}
				} else if field.Names[0].Name == "_" {
					field.Names[0].Name = fmt.Sprintf("_arg_%d", idx)
				}
				return field.Names[0].Name, nil
			}
		}
	}

	// We haven't found anything...
	return "", nil
}
