// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"regexp"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type hasDirective string

func HasDirective(name string) hasDirective {
	return hasDirective(name)
}

func (d hasDirective) Matches(node *node.Chain) bool {
	re := regexp.MustCompile(fmt.Sprintf(`\s*//%s(?:\s.*)?$`, regexp.QuoteMeta(string(d))))
	for _, dec := range node.Node.Decorations().Start.All() {
		if re.MatchString(dec) {
			return true
		}
	}

	if _, isSpec := node.Node.(dst.Spec); isSpec {
		// This is a spec (variable, const, import), so we need to also check the parent for directives!
		return d.Matches(node.Parent())
	}

	return false
}

func (hasDirective) ImpliesImported() []string {
	return nil
}

func (d hasDirective) AsCode() jen.Code {
	return jen.Qual(pkgPath, "HasDirective").Call(jen.Lit(string(d)))
}

func init() {
	unmarshalers["directive"] = func(node *yaml.Node) (Point, error) {
		var name string
		if err := node.Decode(&name); err != nil {
			return nil, err
		}
		return HasDirective(name), nil
	}
}
