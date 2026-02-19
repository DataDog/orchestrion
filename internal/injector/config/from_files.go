// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"context"
	"fmt"
	"os"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/yaml"
)

// LoadFromFiles loads config from a pre-resolved list of YAML file paths. The
// extends directives in each file are ignored since the caller is expected to
// provide the fully-resolved (flattened) list of all config files. Builtin
// aspects are always included.
func LoadFromFiles(ctx context.Context, files []string) (_ Config, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "config.LoadFromFiles",
		tracer.Tag("files.count", len(files)),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	var allAspects []*aspect.Aspect
	// Always include builtin aspects.
	allAspects = append(allAspects, builtIn.Aspects()...)

	for _, filename := range files {
		aspects, err := parseYMLAspects(ctx, filename)
		if err != nil {
			return nil, fmt.Errorf("loading %q: %w", filename, err)
		}
		allAspects = append(allAspects, aspects...)
	}

	return &flatConfig{aspects: allAspects}, nil
}

// parseYMLAspects parses a single YAML config file and returns its aspects,
// ignoring extends directives.
func parseYMLAspects(ctx context.Context, filename string) (_ []*aspect.Aspect, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "config.parseYMLAspects",
		tracer.ResourceName(filename),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", filename, err)
	}
	defer file.Close()

	ctx, yamlDec := yaml.NewDecoderContext(ctx, file)
	var yml ymlFile
	if err := yamlDec.DecodeContext(ctx, &yml); err != nil {
		return nil, fmt.Errorf("yaml.Decode %q: %w", filename, err)
	}

	return yml.Aspects, nil
}

// flatConfig is a Config implementation that holds a flat list of aspects.
type flatConfig struct {
	aspects []*aspect.Aspect
}

func (c *flatConfig) Aspects() []*aspect.Aspect { return c.aspects }
func (c *flatConfig) visit(Visitor, string) error {
	// flatConfig doesn't support visiting since it has no tree structure.
	return nil
}
