// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/goccy/go-yaml/ast"
)

type directive string

// Directive matches nodes that are prefaced by a special pragma comment, which
// is a single-line style comment without any blanks between the leading // and
// the directive name. Directives apply to the node they are directly attached
// to, but also to certain nested nodes:
//   - For assignments, it applies to the RHS only; unless it's a declaration
//     assignment (the := token), in which case it also applies to the LHS,
//   - For call expressions, it applies only to the function part (not the
//     arguments)n
//   - For channel send operations, it only applies to the value being sent,
//   - For defer, go, and return statements, it applies to the value side.
func Directive(name string) directive {
	return directive(name)
}

func (directive) PackageMayMatch(_ *may.PackageContext) may.MatchType {
	return may.Unknown
}

func (d directive) FileMayMatch(ctx *may.FileContext) may.MatchType {
	return ctx.FileContains("//" + string(d))
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

	parent := chain.Parent()
	if parent == nil {
		return false
	}

	switch node := parent.Node().(type) {
	// Also check whether the parent carries the directive if it's one of the node types that would
	// typically carry directives that applies to its nested node.
	case *dst.AssignStmt:
		// For assignments, the directive only applies downwards to the RHS, unless it's a declaration,
		// then it also applies to any declared identifier.
		checkParent := chain.PropertyName() == "Rhs"
		checkParent = checkParent || (node.Tok == token.DEFINE && chain.PropertyName() == "Lhs")
		if checkParent && d.matchesChain(parent) {
			return true
		}
	case *dst.CallExpr:
		// For call expressions, the directive only applies to the called function, not its type
		// signature or arguments list.
		if chain.PropertyName() == "Fun" && d.matchesChain(parent) {
			return true
		}
	case *dst.SendStmt:
		// For chanel send statements, the directive only applies to the value being sent, not to the
		// receiving channel.
		if chain.PropertyName() == "Value" && d.matchesChain(parent) {
			return true
		}
	case *dst.DeferStmt, *dst.ExprStmt, *dst.GoStmt, *dst.ReturnStmt:
		// Defer statements, go statements, and return statements all forward the directive to the
		// value(s); and expression statements are just wrappers of expressions, so naturally directives
		// that apply to the statement also apply to the expression.
		if d.matchesChain(parent) {
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
	unmarshalers["directive"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var name string
		if err := yaml.NodeToValueContext(ctx, node, &name); err != nil {
			return nil, err
		}
		return Directive(name), nil
	}
}
