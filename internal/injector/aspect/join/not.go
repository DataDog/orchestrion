// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type not struct {
	jp Point
}

func Not(jp Point) not {
	return not{jp}
}

func (not) ImpliesImported() []string {
	return nil
}

func (n not) Matches(node *node.Chain) bool {
	return !n.jp.Matches(node)
}

func (n not) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Not").Call(n.jp.AsCode())
}

func (n not) RenderHTML() string {
	if jp, ok := n.jp.(oneOf); ok {
		return fmt.Sprintf(`<div class="join-point none-of"><span class="type pill">None of</span>%s</div>`, jp.renderCandidatesHTML())
	}
	return fmt.Sprintf(`<div class="join-point not"><span class="type pill">Not</span><ul><li>%s</li></ul></div>`, n.jp.RenderHTML())
}

func init() {
	unmarshalers["not"] = func(node *yaml.Node) (Point, error) {
		jp, err := FromYAML(node)
		if err != nil {
			return nil, err
		}
		return Not(jp), nil
	}
}
