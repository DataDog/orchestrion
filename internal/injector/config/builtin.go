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

var builtIn = configGo{
	pkgPath: "github.com/DataDog/orchestrion",
	yaml: &configYML{
		aspects: []*aspect.Aspect{
			{
				ID:             "built.WithOrchestrion",
				TracerInternal: true, // This is safe to apply in the tracer itself
				JoinPoint: join.AllOf(
					join.ValueDeclaration(typed.Bool),
					join.OneOf(
						join.DeclarationOf("github.com/DataDog/orchestrion/runtime/built", "WithOrchestrion"),
						join.Directive("orchestrion:enabled"),
						join.Directive("dd:orchestrion-enabled"), // <- Deprecated
					),
				),
				Advice: []advice.Advice{
					advice.AssignValue(
						code.MustTemplate("true", nil, context.GoLangVersion{}),
					),
				},
			},
			{
				ID:             "built.WithOrchestrionVersion",
				TracerInternal: true, // This is safe to apply in the tracer itself
				JoinPoint: join.AllOf(
					join.ValueDeclaration(typed.String),
					join.OneOf(
						join.DeclarationOf("github.com/DataDog/orchestrion/runtime/built", "WithOrchestrionVersion"),
						join.Directive("orchestrion:version"),
					),
				),
				Advice: []advice.Advice{
					advice.AssignValue(
						code.MustTemplate(`{{Version | printf "%q"}}`, nil, context.GoLangVersion{}),
					),
				},
			},
			{
				ID:             "echo.v4.New.IgnoreDirective",
				TracerInternal: true, // This only redirects to a helper with equivalent semantics.
				JoinPoint:      join.FunctionCall("github.com/labstack/echo/v4", "New"),
				Advice: []advice.Advice{
					advice.ReplaceFunction(
						"github.com/DataDog/orchestrion/runtime/echoignore",
						"New",
					),
				},
			},
		},
		name: "<built-in>",
		meta: configYMLMeta{
			name:        "github.com/DataDog/orchestrion/built & //orchestrion: pragmas",
			description: "Provide runtime visibility into whether orchestrion built an application or not",
			icon:        "cog",
			caveats: "This aspect allows introducing conditional logic based on whether" +
				"Orchestrion has been used to instrument an application or not. This should" +
				"generally be avoided, but can be useful to ensure the application (or tests)" +
				"is running with instrumentation.",
		},
	},
}
