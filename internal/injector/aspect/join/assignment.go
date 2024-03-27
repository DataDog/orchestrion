// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type assignmentOf struct {
	value Point
}

func AssignmentOf(value Point) *assignmentOf {
	return &assignmentOf{value: value}
}

func (i *assignmentOf) Matches(chain *node.Chain) bool {
	stmt, ok := node.As[*dst.AssignStmt](chain)
	if !ok {
		return false
	}

	for idx, rhs := range stmt.Rhs {
		if i.value.Matches(chain.Child(rhs, chain.ImportPath(), "Rhs", idx)) {
			return true
		}
	}
	return false
}

func (i *assignmentOf) AsCode() jen.Code {
	return jen.Qual(pkgPath, "AssignmentOf").Call(i.value.AsCode())
}

func init() {
	unmarshalers["assignment-of"] = func(node *yaml.Node) (Point, error) {
		value, err := FromYAML(node)
		if err != nil {
			return nil, err
		}
		return AssignmentOf(value), nil
	}
}
