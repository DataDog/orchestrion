// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
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

func (a injectDeclarations) Apply(ctx context.Context, chain *node.Chain, _ *dstutil.Cursor) (bool, error) {
	decls, err := a.template.CompileDeclarations(ctx, chain)
	if err != nil {
		return false, err
	}

	file, ok := node.Find[*dst.File](chain)
	if !ok {
		return false, errors.New("cannot inject source file: no *dst.File in context")
	}

	file.Decls = append(file.Decls, decls...)

	if len(a.links) > 0 {
		refMap, found := typed.ContextValue[*typed.ReferenceMap](ctx)
		if !found {
			return true, errors.New("unable to register link requirements, no *typed.ReferenceMap in context")
		}
		refMap.AddImport(file, "unsafe") // We use go:linkname so we have an implicit dependency on unsafe.
		for _, link := range a.links {
			refMap.AddLink(file, link)
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
	buf.WriteString("<div class=\"advice inject-declarations\">\n")
	buf.WriteString("  <div class=\"type\">Introduce new declarations:\n")
	buf.WriteString(a.template.RenderHTML())
	buf.WriteString("\n  </div>\n")
	if len(a.links) > 0 {
		buf.WriteString("  <div class=\"type\">Record link-time dependencies on:\n")
		buf.WriteString("    <ul>\n")
		for _, link := range a.links {
			buf.WriteString(fmt.Sprintf("      <li>{{<godoc %q>}}</li>\n", link))
		}
		buf.WriteString("    </ul>\n")
		buf.WriteString("  </div>\n")
	}
	buf.WriteString("</div>\n")

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
