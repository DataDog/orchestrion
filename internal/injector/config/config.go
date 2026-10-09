// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package config contains APIs used to work with injector configuration files,
// which are formed by [FilenameOrchestrionToolGo] and [FilenameOrchestrionYML] files.
package config

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// PackageLoader resolves package patterns. The string argument is the
// directory the patterns are resolved from: it selects which module is
// "main" for the resolution, and therefore which `replace` directives and
// which `go.sum` apply. It must always be the consuming module's directory,
// never the directory of the package the patterns were found in.
type PackageLoader = func(context.Context, string, ...string) ([]*packages.Package, error)

// HasConfig determines whether the specified package contains injector
// configuration, and optionally validates it. Configuration files are read
// from pkg's own directory, but import paths found in them are resolved from
// dir, which must be the directory of the module consuming pkg (the main
// module) -- never pkg's own directory, which for a module-cache dependency
// would apply that dependency's `replace` directives. If the [PackageLoader]
// is nil, a default implementation is used.
func HasConfig(ctx context.Context, pkgLoader PackageLoader, dir string, pkg *packages.Package, validate bool) (bool, error) {
	if packageRoot(pkg) == "" {
		if err := unresolvedError(pkg); err != nil {
			// The package failed to resolve entirely; we cannot tell whether it
			// provides configuration, so this must not be reported as "no config".
			return false, err
		}
		// Directory resolved and provably holds no Go source: no configuration.
		return false, nil
	}

	l := NewLoader(pkgLoader, dir, validate)
	cfg, err := l.loadGoPackage(ctx, pkg)
	if err != nil {
		return false, err
	}

	return cfg.yaml != nil || len(cfg.imports) != 0, nil
}

// Loader is a facility to load configuration from available sources.
type Loader struct {
	pkgLoader PackageLoader
	loaded    map[string]struct{}
	files     FileDigests
	dir       string
	validate  bool
}

func defaultPackageLoader(ctx context.Context, dir string, patterns ...string) ([]*packages.Package, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "Load",
		tracer.ServiceName("golang.org/x/tools/go/packages"),
		tracer.ResourceName(strings.Join(patterns, " ")),
	)
	defer span.Finish()

	cfg := &packages.Config{
		Context: ctx,
		Dir:     dir,
		Mode:    packages.NeedName | packages.NeedFiles,
		// Neutralize any `-toolexec` inherited from GOFLAGS: resolving
		// configuration must never recurse into orchestrion itself.
		BuildFlags: []string{"-toolexec="},
	}
	return packages.Load(cfg, patterns...)
}

// NewLoader creates a new [Loader] in the specified directory.
//
//	If the [PackageLoader] is nil, a default implementation is used.
//
// The directory is used to resolve relative paths and must be a valid Go
// package directory, meaning it must contain at least one `.go` file. If
// [Loader.validate] is true, the YAML documents will be validated against the
// JSON schema.
func NewLoader(pkgLoader PackageLoader, dir string, validate bool) *Loader {
	if pkgLoader == nil {
		pkgLoader = defaultPackageLoader
	}
	return &Loader{
		pkgLoader: pkgLoader,
		loaded:    make(map[string]struct{}),
		files:     make(FileDigests),
		dir:       dir,
		validate:  validate,
	}
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

func (l *Loader) packages(ctx context.Context, patterns ...string) ([]*packages.Package, error) {
	return l.pkgLoader(ctx, l.dir, patterns...)
}

// FileDigests associates configuration files with a digest of their content, or with nil if they
// do not exist.
type FileDigests map[string]*[sha256.Size]byte

// Files returns the configuration files this loader read or looked for, associated with a digest
// of the content that it read from them (or with nil if they did not exist).
func (l *Loader) Files() FileDigests {
	return l.files
}

// Unchanged reports whether each file still has the recorded content, or still does not exist (in a
// directory that still exists).
func (d FileDigests) Unchanged() bool {
	for filename, digest := range d {
		data, err := os.ReadFile(filename)
		switch {
		case err == nil:
			if digest == nil || sha256.Sum256(data) != *digest {
				return false
			}
		case errors.Is(err, fs.ErrNotExist):
			// Files are looked for in package directories, which existed at the time. If the directory
			// no longer exists (or is now a file, which Windows also reports as fs.ErrNotExist for the
			// files it would contain), the configuration loaded from it may have changed.
			if digest != nil || !dirExists(filepath.Dir(filename)) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// readFile reads the named file, and records a digest of its content (or the fact it does not
// exist) in the loader's [Loader.Files].
func (l *Loader) readFile(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	switch {
	case err == nil:
		digest := sha256.Sum256(data)
		l.files[filename] = &digest
	case errors.Is(err, fs.ErrNotExist):
		l.files[filename] = nil
	}
	return data, err
}
