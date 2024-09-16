// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"fmt"
	"sort"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/datadog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type injectDeclarations struct {
	template code.Template
	links    []string
}

// InjectDeclarations merges all declarations in the provided source file into the current file. The package name of both
// original & injected files must match.
func InjectDeclarations(template code.Template, links []string) injectDeclarations {
	return injectDeclarations{template, links}
}

func (a injectDeclarations) Apply(ctx context.AdviceContext) (bool, error) {
	decls, err := a.template.CompileDeclarations(ctx)
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
	if len(a.links) > 0 {
		ctx.AddImport("unsafe", "_") // For go:linkname
		for _, link := range a.links {
			ctx.AddLink(link)
		}
	}

	return true, nil
}

func (a injectDeclarations) AsCode() jen.Code {
	return jen.Qual(pkgPath, "InjectDeclarations").Call(
		a.template.AsCode(),
		jen.Index().String().ValuesFunc(func(g *jen.Group) {
			sort.Strings(a.links)
			for _, link := range a.links {
				g.Line().Lit(link)
			}
			g.Line()
		}),
	)
}

func (a injectDeclarations) AddedImports() []string {
	return a.links
}

func (a injectDeclarations) RenderHTML() string {
	var buf strings.Builder
	_, _ = buf.WriteString("<div class=\"advice inject-declarations\">\n")
	_, _ = buf.WriteString("  <div class=\"type\">Introduce new declarations:\n")
	_, _ = buf.WriteString(a.template.RenderHTML())
	_, _ = buf.WriteString("\n  </div>\n")
	if len(a.links) > 0 {
		_, _ = buf.WriteString("  <div class=\"type\">Record link-time dependencies on:\n")
		_, _ = buf.WriteString("    <ul>\n")
		for _, link := range a.links {
			_, _ = buf.WriteString(fmt.Sprintf("      <li>{{<godoc %q>}}</li>\n", link))
		}
		_, _ = buf.WriteString("    </ul>\n")
		_, _ = buf.WriteString("  </div>\n")
	}
	_, _ = buf.WriteString("</div>\n")

	return buf.String()
}

func init() {
	unmarshalers["inject-declarations"] = func(node *yaml.Node) (Advice, error) {
		var config struct {
			Template code.Template `yaml:",inline"`
			Links    []string
		}
		if err := node.Decode(&config); err != nil {
			return nil, err
		}

		return InjectDeclarations(config.Template, config.Links), nil
	}
}
