// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"gopkg.in/yaml.v3"
)

type directive string

func Directive(name string) directive {
	return directive(name)
}

func (d directive) Matches(ctx context.AspectContext) bool {
	return d.matchesChain(ctx.Chain())
}

func (d directive) matchesChain(chain *context.NodeChain) bool {
	for _, dec := range chain.Node().Decorations().Start.All() {
		if d.matches(dec) {
			return true
		}
	}

	// If this is a spec (variable, const, import), so we need to also check the parent for directives!
	if _, isSpec := chain.Node().(dst.Spec); isSpec {
		if parent := chain.Parent(); parent != nil && d.matchesChain(parent) {
			return true
		}
	}

	// If the parent is an assignment statement, so we also check it for directives.
	if parent := chain.Parent(); parent != nil {
		if _, isAssign := parent.Node().(*dst.AssignStmt); isAssign && d.matchesChain(parent) {
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
	r, size := utf8.DecodeRuneInString(rest)
	return size == 0 || unicode.IsSpace(r)
}

func (directive) ImpliesImported() []string {
	return nil
}

func (d directive) Hash(h *fingerprint.Hasher) error {
	return h.Named("directive", fingerprint.String(d))
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
