// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type testMain bool

// TestMain matches only nodes in ASTs in files that either are (if true), or
// are not (if false) part of a synthetic test main package.
func TestMain(v bool) testMain {
	return testMain(v)
}

func (t testMain) Matches(ctx context.AspectContext) bool {
	return ctx.TestMain() == bool(t)
}

func (testMain) ImpliesImported() []string {
	return nil
}

func (t testMain) AsCode() jen.Code {
	return jen.Qual(pkgPath, "TestMain").Call(jen.Lit(bool(t)))
}

func init() {
	unmarshalers["test-main"] = func(node *yaml.Node) (Point, error) {
		var val bool
		if err := node.Decode(&val); err != nil {
			return nil, err
		}
		return TestMain(val), nil
	}
}
