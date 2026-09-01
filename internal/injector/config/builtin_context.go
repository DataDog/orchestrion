// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/join"
	"github.com/DataDog/orchestrion/internal/injector/typed"
)

// contextPackage is the import path of the goroutine context propagation
// package woven into `go` statements and `func main()` below. It is kept
// as a constant here to guarantee the aspects below stay in sync with the
// package they inject calls into.
const contextPackage = "github.com/DataDog/orchestrion/runtime/context"

// glsFieldName is the name of the field this package's aspects add to
// runtime.g, and the getter/setter symbols linked to it. It is distinct
// from the `__dd_gls_v2` field historically added by dd-trace-go's own
// (external) GLS aspect, so the two can coexist on the same build.
const glsFieldName = "__dd_orchestrion_ctx"

// contextPropagationAspects wires up runtime.context's goroutine-crossing
// context propagation: a storage slot on runtime.g, scrubbed when a
// goroutine exits and reused by a subsequent one; a rewrite of every `go`
// statement to snapshot/install that storage across the goroutine boundary;
// and a call at the start of `func main()` to seed the main goroutine's
// storage from each registered Hooks[T].Main().
var contextPropagationAspects = []*aspect.Aspect{
	{
		ID:             "context.gls",
		TracerInternal: true,
		JoinPoint:      join.StructDefinition(typed.TypeName{ImportPath: "runtime", Name: "g"}),
		Advice: []advice.Advice{
			advice.AddStructField(glsFieldName, typed.Any),
			advice.AddBlankImport("unsafe"), // for go:linkname
			advice.InjectDeclarations(
				code.MustTemplate(`
//go:linkname __dd_orchestrion_ctx_get __dd_orchestrion_ctx_get
var __dd_orchestrion_ctx_get = func() any {
	return getg().m.curg.`+glsFieldName+`
}

//go:linkname __dd_orchestrion_ctx_set __dd_orchestrion_ctx_set
var __dd_orchestrion_ctx_set = func(val any) {
	getg().m.curg.`+glsFieldName+` = val
}
`, nil, context.GoLangVersion{}),
				nil,
			),
		},
	},
	{
		ID:             "context.gls.scrub",
		TracerInternal: true,
		JoinPoint: join.AllOf(
			join.ImportPath("runtime"),
			join.FunctionBody(join.Function(join.Name("goexit1"))),
		),
		Advice: []advice.Advice{
			// A goroutine's runtime.g is recycled for a future goroutine once
			// this one exits; the slot must be cleared or the next goroutine
			// spawned on this g would inherit stale context.
			advice.PrependStmts(code.MustTemplate("getg()."+glsFieldName+" = nil", nil, context.GoLangVersion{})),
		},
	},
	{
		ID:             "context.goroutine",
		TracerInternal: true,
		JoinPoint: join.AllOf(
			// runtime itself cannot import non-runtime packages, and internal/*
			// GOROOT packages have similarly restricted import graphs; `go`
			// statements there are left untouched.
			join.Not(join.ImportPath("runtime")),
			join.Not(join.PackageFilter(false, "internal/**")),
			join.GoStatementCall(),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"orchestrionctx.WrapGoroutine(func() { {{ . }} })()",
				map[string]string{"orchestrionctx": contextPackage},
				context.GoLangVersion{},
			)),
		},
	},
	{
		ID:             "context.main",
		TracerInternal: true,
		JoinPoint: join.AllOf(
			join.ImportPath("main"),
			join.FunctionBody(join.Function(join.Name("main"))),
		),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"orchestrionctx.Bootstrap()",
				map[string]string{"orchestrionctx": contextPackage},
				context.GoLangVersion{},
			)),
		},
	},
}

func init() {
	builtIn.yaml.aspects = append(builtIn.yaml.aspects, contextPropagationAspects...)
}
