// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"sort"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type configuration map[string]string

func Configuration(requirements map[string]string) configuration {
	return configuration(requirements)
}

func (configuration) ImpliesImported() []string {
	return nil
}

func (jp configuration) Matches(chain *node.Chain) bool {
	for k, v := range jp {
		cfg, found := chain.Config(k)
		if !found || cfg != v {
			return false
		}
	}
	return true
}

func (jp configuration) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Configuration").Call(jen.Map(jen.String()).String().ValuesFunc(func(g *jen.Group) {
		keys := make([]string, 0, len(jp))
		for k := range jp {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			g.Line().Lit(k).Op(":").Lit(jp[k])
		}

		g.Line().Empty()
	}))
}

func (jp configuration) RenderHTML() string {
	var buf strings.Builder

	buf.WriteString("<div class=\"join-point configuration\">\n")
	buf.WriteString("  <span class=\"type pill\">Configuration</span>\n")
	buf.WriteString("  <ul>\n")
	for k, v := range jp {
		buf.WriteString("    <li class=\"flex\">\n")
		buf.WriteString("      <span class=\"type\">")
		buf.WriteString(k)
		buf.WriteString("</span>\n")
		buf.WriteString("      <code>\n")
		buf.WriteString(v)
		buf.WriteString("      </code>\n")
		buf.WriteString("    </li>\n")
	}
	buf.WriteString("  </ul>\n")
	buf.WriteString("</div>\n")

	return buf.String()
}

func init() {
	unmarshalers["configuration"] = func(node *yaml.Node) (Point, error) {
		var c configuration
		return c, node.Decode(&c)
	}
}
