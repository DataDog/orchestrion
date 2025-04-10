// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/goccy/go-yaml/ast"

	_ "embed" // For go:embed
)

type configuration map[string]string

func Configuration(requirements map[string]string) configuration {
	return configuration(requirements)
}

func (configuration) ImpliesImported() []string {
	return nil
}

func (configuration) PackageMayMatch(_ *may.PackageContext) may.MatchType {
	return may.Unknown
}

func (configuration) FileMayMatch(_ *may.FileContext) may.MatchType {
	return may.Unknown
}

func (jp configuration) Matches(ctx context.AspectContext) bool {
	for k, v := range jp {
		cfg, found := ctx.Config(k)
		if !found || cfg != v {
			return false
		}
	}
	return true
}

func (jp configuration) Hash(h *fingerprint.Hasher) error {
	return h.Named("configuration", fingerprint.Map(jp, func(k string, v string) (string, fingerprint.String) { return k, fingerprint.String(v) }))
}

func init() {
	unmarshalers["configuration"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var c configuration
		return c, yaml.NodeToValueContext(ctx, node, &c)
	}
}
