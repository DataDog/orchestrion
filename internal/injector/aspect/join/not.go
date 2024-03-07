// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"bytes"
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

func (n not) ToHTML() string {
	if oneOf, ok := n.jp.(oneOf); ok {
		buf := &bytes.Buffer{}
		buf.WriteString("<strong>None of</strong> the following:\n")
		buf.WriteString("<ul>\n")
		for _, jp := range oneOf {
			fmt.Fprintf(buf, "<li>%s</li>\n", jp.ToHTML())
		}
		buf.WriteString("</ul>\n")
		return buf.String()
	}
	return fmt.Sprintf("<strong>Not</strong>:<div>%s</div>", n.jp.ToHTML())
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
