// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/goccy/go-yaml/ast"
)

const FilenameOrchestrionYML = "orchestrion.yml"

// loadYMLFile loads configuration from the specified directory.
func (l *Loader) loadYMLFile(ctx context.Context, dir string, name string) (_ *configYML, err error) {
	filename := name
	if !filepath.IsAbs(name) {
		filename = filepath.Join(dir, name)
	}
	if !l.markLoaded(filename) {
		// Already loaded, ignoring...
		return nil, nil
	}

	span, ctx := tracer.StartSpanFromContext(ctx, "config.loadYMLFile",
		tracer.ResourceName(filename),
	)
	defer func() {
		spanErr := err
		if errors.Is(err, fs.ErrNotExist) {
			spanErr = nil
		}
		span.Finish(tracer.WithError(spanErr))
	}()

	yml, err := l.parseYMLFile(ctx, filename)
	if err != nil {
		return nil, err
	}

	dir = filepath.Dir(filename)
	extends := make([]Config, 0, len(yml.Extends))
	for _, ext := range yml.Extends {
		extFilename := filepath.Join(dir, ext)

		if stat, err := os.Stat(extFilename); err != nil {
			return nil, maskErrNotExist(err)
		} else if stat.IsDir() {
			pkgs, err := l.packages(ctx, extFilename)
			if err != nil {
				return nil, fmt.Errorf("extends %q: %w", ext, err)
			}
			if len(pkgs) != 1 {
				// This is not supposed to happen if `err == nil`.
				panic(fmt.Errorf("extends %q: no package returned by packages.Load(%q)", ext, l.dir))
			}

			cfg, err := l.loadGoPackage(ctx, pkgs[0])
			if err != nil {
				return nil, maskErrNotExist(err)
			}
			if cfg.empty() {
				// Empty, nothing to do...
				continue
			}
			extends = append(extends, cfg)
			continue
		}

		cfg, err := l.loadYMLFile(ctx, dir, ext)
		if err != nil {
			return nil, maskErrNotExist(err)
		}
		if cfg.empty() {
			// Empty, nothing to do...
			continue
		}
		extends = append(extends, cfg)
	}

	cfg := &configYML{name: name, extends: extends, aspects: yml.Aspects}
	cfg.meta.name = yml.Meta.Name
	cfg.meta.description = yml.Meta.Description
	cfg.meta.icon = yml.Meta.Icon
	cfg.meta.caveats = yml.Meta.Caveats

	return cfg, nil
}

type (
	configYML struct {
		extends []Config
		aspects []*aspect.Aspect
		name    string
		meta    configYMLMeta
	}
	configYMLMeta struct {
		name        string
		description string
		icon        string
		caveats     string
	}
)

func (c *configYML) Aspects() []*aspect.Aspect {
	if c == nil {
		return nil
	}

	var res []*aspect.Aspect
	for _, ext := range c.extends {
		res = append(res, ext.Aspects()...)
	}
	res = append(res, c.aspects...)

	return res
}

func (c *configYML) visit(v Visitor, pkgPath string) error {
	if c == nil {
		return nil
	}

	if err := v(c, pkgPath); err != nil {
		return err
	}

	for _, ext := range c.extends {
		if err := ext.visit(v, pkgPath); err != nil {
			return err
		}
	}

	return nil
}

func (c *configYML) empty() bool {
	return c == nil || (len(c.extends) == 0 && len(c.aspects) == 0)
}

type ymlFile struct {
	Aspects []*aspect.Aspect
	Extends []string
	Meta    struct {
		Name        string
		Description string
		Icon        string // Optional
		Caveats     string // Optional
	}
}

func (l *Loader) parseYMLFile(ctx context.Context, filename string) (*ymlFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", filename, err)
	}
	defer file.Close()

	// In validation mode, we will pre-parse the YAML into a [yaml.Node] tree,
	// which can then be cheaply decoded into a value type that validation
	// supports; and then re-decoded into the actual data structure.
	// This dance is significantly cheaper (both in time & allocations) than doing
	// a full blown [yaml.Decoder.Decode] twice (as it internally transits through
	// the [yaml.Node] representation anyway).
	ctx, yamlDec := yaml.NewDecoderContext(ctx, file)
	var dec interface {
		DecodeContext(context.Context, any) error
	} = yamlDec
	if l.validate {
		var node ast.Node
		if err := dec.DecodeContext(ctx, &node); err != nil {
			return nil, fmt.Errorf("yaml.Decode %q -> yaml.Node: %w", filename, err)
		}
		dec = decodedNode{yamlDec, node}

		var simple map[string]any
		if err := dec.DecodeContext(ctx, &simple); err != nil {
			return nil, fmt.Errorf("yaml.Decode %q -> map[string]any: %w", filename, err)
		}

		if err := ValidateObject(simple); err != nil {
			return nil, fmt.Errorf("validate %q: %w", filename, err)
		}
	}

	var yml ymlFile
	if err := dec.DecodeContext(ctx, &yml); err != nil {
		return nil, fmt.Errorf("yaml.Decode %q: %w", filename, err)
	}

	return &yml, nil
}

type decodedNode struct {
	*yaml.Decoder
	ast.Node
}

func (n decodedNode) DecodeContext(ctx context.Context, out any) error {
	return n.Decoder.DecodeFromNodeContext(ctx, n.Node, out)
}

// maskErrNotExist intentionally "breaks" the error chaining if the provided
// error is an [fs.ErrNotExist] so that the returned error is not
// [fs.ErrNotExist]. Otherwise, returns the original error unmodified.
func maskErrNotExist(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%v", err)
	}
	return err
}
