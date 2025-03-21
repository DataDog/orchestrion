// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package config contains APIs used to work with injector configuration files,
// which are formed by [FilenameOrchestrionToolGo] and [FilenameOrchestrionYML] files.
package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"golang.org/x/tools/go/packages"
)

// Config represents an injector's configuration. It can be obtained using
// [Loader.Load].
type Config interface {
	// Aspects returns all aspects defined in this configuration in a single list.
	Aspects() []*aspect.Aspect

	visit(Visitor, string) error
}

// HasConfig determines whether the specified package contains injector
// configuration, and optionally validates it.
func HasConfig(ctx context.Context, pkg *packages.Package, validate bool) (bool, error) {
	root := packageRoot(pkg)
	if root == "" {
		// It contains no .go file, so it can't contain configuration.
		return false, nil
	}

	l := NewLoader(root, validate)
	cfg, err := l.loadGoPackage(ctx, pkg)
	if err != nil {
		return false, err
	}

	return cfg.yaml != nil || len(cfg.imports) != 0, nil
}

// Loader is a facility to load configuration from available sources.
type Loader struct {
	loaded   map[string]struct{}
	dir      string
	validate bool
}

// NewLoader creates a new [Loader] in the specified directory. The directory
// is used to resolve relative paths and must be a valid Go package directory,
// meaning it must contain at least one `.go` file. If [Loader.validate] is
// true, the YAML documents will be validated against the JSON schema.
func NewLoader(dir string, validate bool) *Loader {
	return &Loader{loaded: make(map[string]struct{}), dir: dir, validate: validate}
}

// Load proceeds to load the configuration from this loader's directory.
func (l *Loader) Load(ctx context.Context) (_ Config, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "Load",
		tracer.ServiceName("github.com/DataDog/orchestrion/internal/injector/config"),
		tracer.ResourceName(l.dir),
		tracer.Tag("validate", l.validate),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	pkgs, err := l.packages(ctx, l.dir)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		// This is not supposed to happen if `err == nil`.
		panic(fmt.Errorf("no package returned by packages.Load(%q)", l.dir))
	}

	return l.loadGoPackage(ctx, pkgs[0])
}

// markLoaded marks the specified file as loaded. Return true if the file was
// not already marked previously.
func (l *Loader) markLoaded(filename string) bool {
	if _, found := l.loaded[filename]; found {
		return false
	}
	l.loaded[filename] = struct{}{}
	return true
}

func (l *Loader) packages(ctx context.Context, patterns ...string) (_ []*packages.Package, err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "Load",
		tracer.ServiceName("golang.org/x/tools/go/packages"),
		tracer.ResourceName(strings.Join(patterns, " ")),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	cfg := packages.Config{
		BuildFlags: []string{"-toolexec="},
		Dir:        l.dir,
		Mode:       packages.NeedName | packages.NeedFiles,
	}
	return packages.Load(&cfg, patterns...)
}
