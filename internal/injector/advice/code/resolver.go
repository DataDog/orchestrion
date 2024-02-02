// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/join"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type resolver struct {
	*dstutil.Cursor
}

type DirectiveArgument struct {
	Name  string
	Value string
}

var spaces = regexp.MustCompile(`\s+`)

func (r *resolver) DirectiveArgs(name string) (args []DirectiveArgument) {
	nodes := []dst.Node{r.Node(), nil}[0:1]
	switch parent := r.Parent().(type) {
	case *dst.AssignStmt:
		nodes = append(nodes, r.Parent())
	case *dst.FuncDecl:
		if r.Node() == parent.Body {
			nodes = append(nodes, r.Parent())
		}
	case *dst.FuncLit:
		if r.Node() == parent.Body {
			nodes = append(nodes, r.Parent())
		}
	}

	prefix := "//" + name
	for _, node := range nodes {
		for _, dec := range node.Decorations().Start {
			parts := spaces.Split(dec, -1)
			if len(parts) == 0 || parts[0] != prefix {
				continue
			}
			parts = parts[1:]
			args = make([]DirectiveArgument, len(parts))
			for idx, part := range parts {
				key, value, _ := strings.Cut(part, ":")
				args[idx] = DirectiveArgument{key, value}
			}
		}
	}

	return
}

func (r *resolver) FindArg(typename string) (string, error) {
	tn, err := join.NewTypeName(typename)
	if err != nil {
		return "", err
	}

	var fnType *dst.FuncType
	if fnDecl, ok := r.Node().(*dst.FuncDecl); ok {
		fnType = fnDecl.Type
	} else if fnDecl, ok := r.Parent().(*dst.FuncDecl); ok {
		fnType = fnDecl.Type
	} else if fnLit, ok := r.Node().(*dst.FuncLit); ok {
		fnType = fnLit.Type
	} else if fnLit, ok := r.Parent().(*dst.FuncLit); ok {
		fnType = fnLit.Type
	} else {
		return "", errors.New("no function is available in this context")
	}

	for idx, field := range fnType.Params.List {
		if !tn.Matches(field.Type) {
			continue
		}
		if len(field.Names) == 0 {
			field.Names = []*dst.Ident{dst.NewIdent(fmt.Sprintf("__arg_%d", idx))}
		} else if field.Names[0].Name == "_" {
			field.Names[0].Name = fmt.Sprintf("__arg_%d", idx)
		}
		return field.Names[0].Name, nil
	}

	return "", fmt.Errorf("no argument matches %q", typename)
}
