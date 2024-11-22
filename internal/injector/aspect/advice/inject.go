// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"gopkg.in/yaml.v3"
)

type injectDeclarations struct {
	Template *code.Template
	Links    []string
}

// InjectDeclarations merges all declarations in the provided source file into the current file. The package name of both
// original & injected files must match.
func InjectDeclarations(template *code.Template, links []string) injectDeclarations {
	return injectDeclarations{template, links}
}

func (a injectDeclarations) Apply(ctx context.AdviceContext) (bool, error) {
	decls, err := a.Template.CompileDeclarations(ctx)
	if err != nil {
		return false, err
	}

	if len(decls) == 0 {
		return false, nil
	}

	// Add the declarations to the file
	file := ctx.File()
	file.Decls = append(file.Decls, decls...)

	// Register any link-time dependencies that were declared...
	if len(a.Links) > 0 {
		ctx.AddImport("unsafe", "_") // For go:linkname
		for _, link := range a.Links {
			ctx.AddLink(link)
		}
	}

	ctx.EnsureMinGoLang(a.Template.Lang)

	return true, nil
}

func (a injectDeclarations) Hash(h *fingerprint.Hasher) error {
	return h.Named(
		"inject-declarations",
		fingerprint.Cast(a.Links, func(s string) fingerprint.String { return fingerprint.String(s) }),
		a.Template,
	)
}

func (a injectDeclarations) AddedImports() []string {
	return append(a.Template.AddedImports(), a.Links...)
}

func init() {
	unmarshalers["inject-declarations"] = func(node *yaml.Node) (Advice, error) {
		var config struct {
			Template *code.Template `yaml:",inline"`
			Links    []string
		}
		if err := node.Decode(&config); err != nil {
			return nil, err
		}

		return InjectDeclarations(config.Template, config.Links), nil
	}
}
