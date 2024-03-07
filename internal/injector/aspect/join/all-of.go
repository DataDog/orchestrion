// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type allOf []Point

func AllOf(requirements ...Point) allOf {
	return requirements
}

func (o allOf) ImpliesImported() (list []string) {
	for _, jp := range o {
		list = append(list, jp.ImpliesImported()...)
	}
	return
}

func (o allOf) Matches(node *node.Chain) bool {
	for _, candidate := range o {
		if !candidate.Matches(node) {
			return false
		}
	}
	// Never matches if there is no requirement
	return len(o) > 0
}

func (o allOf) AsCode() jen.Code {
	if len(o) == 1 {
		return o[0].AsCode()
	}

	return jen.Qual(pkgPath, "AllOf").CallFunc(func(g *jen.Group) {
		if len(o) > 0 {
			for _, candidate := range o {
				g.Line().Add(candidate.AsCode())
			}
			g.Line().Empty()
		}
	})
}

func (o allOf) ToHTML() string {
	buf := &strings.Builder{}
	fmt.Fprintln(buf, "<strong>All of</strong> the following:")
	fmt.Fprintln(buf, "<ul>")
	for _, candidate := range o {
		fmt.Fprintf(buf, "<li>%s</li>", candidate.ToHTML())
	}
	fmt.Fprintln(buf, "</ul>")
	return buf.String()
}

func init() {
	unmarshalers["all-of"] = func(node *yaml.Node) (Point, error) {
		var nodes []yaml.Node
		if err := node.Decode(&nodes); err != nil {
			return nil, err
		}

		if len(nodes) == 1 {
			pt, err := FromYAML(&nodes[0])
			return pt, err
		}

		requirements := make([]Point, len(nodes))
		for i, n := range nodes {
			var err error
			if requirements[i], err = FromYAML(&n); err != nil {
				return nil, err
			}
		}
		return AllOf(requirements...), nil
	}
}
