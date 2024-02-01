// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package at

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type assignmentOf struct {
	value InjectionPoint
}

func AssignmentOf(value InjectionPoint) *assignmentOf {
	return &assignmentOf{value: value}
}

func (i *assignmentOf) Matches(c *dstutil.Cursor) bool {
	return i.matchesNode(c.Node(), c.Parent())
}

func (i *assignmentOf) matchesNode(node dst.Node, parent dst.Node) bool {
	stmt, ok := node.(*dst.AssignStmt)
	if !ok {
		return false
	}

	for _, rhs := range stmt.Rhs {
		if i.value.matchesNode(rhs, stmt) {
			return true
		}
	}
	return false
}

func init() {
	unmarshalers["assignment-of"] = func(node *yaml.Node) (InjectionPoint, error) {
		value, err := Unmarshal(node)
		if err != nil {
			return nil, err
		}
		return AssignmentOf(value), nil
	}
}
