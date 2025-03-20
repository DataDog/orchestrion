// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"golang.org/x/tools/go/packages"
)

const FilenameOrchestrionToolGo = "orchestrion.tool.go"

var ErrInvalidGoPackage = errors.New("no .go files in package")

// loadGoPackage loads configuration from the specified go package.
func (l *Loader) loadGoPackage(ctx context.Context, pkg *packages.Package) (_ *configGo, err error) {
	// Special-case the `github.com/DataDog/orchestrion` package, we need not
	// parse this one, and should always use the built-in object.
	if pkg.PkgPath == builtIn.pkgPath {
		return &builtIn, nil
	}

	span, ctx := tracer.StartSpanFromContext(ctx, "config.loadGoPackage",
		tracer.ResourceName(pkg.PkgPath),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	root := packageRoot(pkg)
	if root == "" {
		// This might be explained by a package-level loading error... We only check
		// here because "all .go files are excluded by build constraints" is one
		// such error that we typically ignore.
		if pkg.Errors != nil {
			var err error
			for _, e := range pkg.Errors {
				var innerErr error = e
				if e.Kind == packages.ListError && strings.Contains(e.Msg, "no Go files in") { // Workaround poor error typing in packages.Load
					innerErr = fmt.Errorf("no Go files found, was expecting at least orchestrion.tool.go: %w", e)
				}
				err = errors.Join(err, innerErr)
			}
			return nil, fmt.Errorf("in %q: %w", pkg.ID, err)
		}

		return nil, fmt.Errorf("%q: %w", pkg.PkgPath, ErrInvalidGoPackage)
	}

	toolFile := filepath.Join(root, FilenameOrchestrionToolGo)
	imports, err := l.loadGoFile(ctx, toolFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	ymlCfg, err := l.loadYMLFile(ctx, root, FilenameOrchestrionYML)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return &configGo{imports: imports, yaml: ymlCfg, pkgPath: pkg.PkgPath}, nil
}

// loadGoFile loads configuration from the specified go file. Returns nil if the
// file had already been loaded previously, or if the configuration is empty.
func (l *Loader) loadGoFile(ctx context.Context, filename string) (_ []Config, err error) {
	if !l.markLoaded(filename) {
		return nil, nil
	}

	span, ctx := tracer.StartSpanFromContext(ctx, "config.loadGoFile", tracer.Tag("filename", filename))
	defer func() { span.Finish(tracer.WithError(err)) }()

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", filename, err)
	}
	defer file.Close()

	fset := token.NewFileSet()
	ast, err := parser.ParseFile(fset, filename, file, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", filename, err)
	}

	imports := make([]string, 0, len(ast.Imports))
	for _, spec := range ast.Imports {
		if spec.Path == nil {
			return nil, fmt.Errorf("missing import path at %s", fset.Position(spec.Pos()))
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid import path at %q", fset.Position(spec.Pos()))
		}
		imports = append(imports, path)
	}

	pkgs, err := l.packages(ctx, imports...)
	if err != nil {
		return nil, fmt.Errorf("imports from %q (%q): %w", filename, imports, err)
	}

	cfgs := make([]Config, 0, len(pkgs))
	for _, pkg := range pkgs {
		cfg, err := l.loadGoPackage(ctx, pkg)
		if err != nil {
			return nil, fmt.Errorf("in %q imported from %q: %w", pkg.PkgPath, filename, err)
		}
		if cfg.empty() {
			// Already loaded or empty
			continue
		}
		cfgs = append(cfgs, cfg)
	}

	return cfgs, nil
}

type configGo struct {
	imports []Config
	yaml    *configYML
	pkgPath string
}

func (c *configGo) Aspects() []*aspect.Aspect {
	if c == nil {
		return nil
	}

	var res []*aspect.Aspect
	for _, imp := range c.imports {
		res = append(res, imp.Aspects()...)
	}
	if c.yaml != nil {
		res = append(res, c.yaml.Aspects()...)
	}

	return res
}

func (c *configGo) visit(v Visitor, _ string) error {
	if err := c.yaml.visit(v, c.pkgPath); err != nil {
		return err
	}

	for _, imp := range c.imports {
		if err := imp.visit(v, c.pkgPath); err != nil {
			return err
		}
	}

	return nil
}

func (c *configGo) empty() bool {
	return c == nil || (len(c.imports) == 0 && c.yaml.empty())
}

// packageRoot returns the root directory of the provided package. It must have
// been loaded with the [packages.NeedFiles] mode, which is the case for values
// returned by [Loader.packages].
func packageRoot(pkg *packages.Package) string {
	if len(pkg.GoFiles) > 0 {
		return filepath.Dir(pkg.GoFiles[0])
	}
	if len(pkg.IgnoredFiles) > 0 {
		return filepath.Dir(pkg.IgnoredFiles[0])
	}
	return ""
}
