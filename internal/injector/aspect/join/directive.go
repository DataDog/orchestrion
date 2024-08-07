// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type directive string

func Directive(name string) directive {
	return directive(name)
}

func (d directive) Matches(node *node.Chain) bool {
	for _, dec := range node.Node.Decorations().Start.All() {
		if d.matches(dec) {
			return true
		}
	}

	// If this is a spec (variable, const, import), so we need to also check the parent for directives!
	if _, isSpec := node.Node.(dst.Spec); isSpec && d.Matches(node.Parent()) {
		return true
	}

	// If the parent is an assignment statement, so we also check it for directives.
	if parent := node.Parent(); parent != nil {
		if _, isAssign := parent.Node.(*dst.AssignStmt); isAssign && d.Matches(parent) {
			return true
		}
	}

	return false
}

func (d directive) matches(dec string) bool {
	// Trim leading white space
	rest := strings.TrimLeftFunc(dec, unicode.IsSpace)

	// Check we have a single-line comment
	if !strings.HasPrefix(rest, "//") {
		return false
	}

	// Check the // is followed immediately by the directive name
	rest = rest[2:]
	if !strings.HasPrefix(rest, string(d)) {
		return false
	}

	// If there is something after the directive name, it must be white space
	rest = rest[len(d):]
	rune, size := utf8.DecodeRuneInString(rest)
	return size == 0 || unicode.IsSpace(rune)
}

func (directive) ImpliesImported() []string {
	return nil
}

func (d directive) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Directive").Call(jen.Lit(string(d)))
}

func (d directive) RenderHTML() string {
	return fmt.Sprintf(`<div class="flex join-point directive"><span class="type">Has directive</span><code>//%s</code></div>`, string(d))
}

func init() {
	unmarshalers["directive"] = func(node *yaml.Node) (Point, error) {
		var name string
		if err := node.Decode(&name); err != nil {
			return nil, err
		}
		return Directive(name), nil
	}
}
