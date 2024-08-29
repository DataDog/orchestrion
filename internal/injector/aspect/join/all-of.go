// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"strings"

	"github.com/datadog/orchestrion/internal/injector/aspect/context"
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

func (o allOf) Matches(ctx context.AspectContext) bool {
	for _, candidate := range o {
		if !candidate.Matches(ctx) {
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

func (o allOf) RenderHTML() string {
	var buf strings.Builder
	_, _ = buf.WriteString("<div class=\"join-point all-of\">")
	_, _ = buf.WriteString("  <span class=\"type pill\">All of</span>")
	_, _ = buf.WriteString("  <ul>\n")
	for _, candidate := range o {
		_, _ = buf.WriteString("    <li class=\"candidate\">\n")
		_, _ = buf.WriteString(candidate.RenderHTML())
		_, _ = buf.WriteString("    </li>\n")
	}
	_, _ = buf.WriteString("  </ul>\n")
	_, _ = buf.WriteString("</div>\n")
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
