// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/log"
	"github.com/DataDog/orchestrion/internal/pin"
	"golang.org/x/tools/go/packages"
)

type goSource struct {
	imports []*goSource
	yaml    *yamlSource
	pkgPath string
}

func (l *loader) loadPackage(pkg string, dir string) (*goSource, error) {
	filename := filepath.Join(dir, pin.OrchestrionToolGo)
	if l.markLoaded(filename) {
		return nil, nil
	}

	pkgPath, err := resolveName(pkg, dir)
	if err != nil {
		return nil, fmt.Errorf("resolving import path of %q within %q: %w", pkg, dir, err)
	}
	source := &goSource{
		pkgPath: pkgPath,
	}

	if err := source.loadGoSource(l, dir, filename); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}

	yml, err := l.loadYAMLFromDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	source.yaml = yml

	return source, nil
}

func (s *goSource) loadGoSource(l *loader, dir string, filename string) error {
	fset := token.NewFileSet()
	ast, err := parseAST(filename, fset)
	if err != nil {
		return err
	}

	paths := make([]string, 0, len(ast.Imports))
	for _, spec := range ast.Imports {
		// Those should fail at `parser.ParseFile`; but we check again for safety.
		if spec.Path == nil || spec.Path.Kind != token.STRING {
			return fmt.Errorf("invalid import spec at %s - %w", fset.Position(spec.Pos()), err)
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return fmt.Errorf("invalid import spec at %s - %w", fset.Position(spec.Pos()), err)
		}
		if path == "github.com/DataDog/orchestrion" {
			// We know this package does not need investigating...
			continue
		}
		paths = append(paths, path)
	}

	pkgs, err := packages.Load(
		&packages.Config{
			BuildFlags: []string{"-toolexec="},
			Dir:        dir,
			Logf:       logf,
			Mode:       packages.NeedName | packages.NeedFiles,
		},
		paths...,
	)
	if err != nil {
		return fmt.Errorf("loading packages %q: %w", paths, err)
	}

	s.imports = make([]*goSource, 0, len(pkgs))
	for _, pkg := range pkgs {
		var pkgRoot string
		if len(pkg.GoFiles) > 0 {
			pkgRoot = filepath.Dir(pkg.GoFiles[0])
		} else if len(pkg.IgnoredFiles) > 0 {
			pkgRoot = filepath.Dir(pkg.IgnoredFiles[0])
		} else {
			// The package contains no files; so we cannot find YAML config in there.
			continue
		}

		cfg, err := l.loadPackage(pkg.PkgPath, pkgRoot)
		if err != nil {
			return fmt.Errorf("go package %q: %w", pkg.PkgPath, err)
		}
		if cfg == nil || cfg.Empty() {
			continue
		}
		s.imports = append(s.imports, cfg)
	}

	return nil
}

func (s *goSource) Aspects() []aspect.Aspect {
	var aspects []aspect.Aspect

	for _, cfg := range s.imports {
		aspects = append(aspects, cfg.Aspects()...)
	}
	if s.yaml != nil {
		aspects = append(aspects, s.yaml.Aspects()...)
	}

	return aspects
}

func (s *goSource) Empty() bool {
	return len(s.imports) == 0 && (s.yaml == nil || s.yaml.Empty())
}

func parseAST(filename string, fset *token.FileSet) (*ast.File, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", filename, err)
	}
	defer file.Close()

	ast, err := parser.ParseFile(fset, filename, file, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", filename, err)
	}

	return ast, nil
}

func resolveName(pkg string, dir string) (string, error) {
	if pkg != "." && !strings.HasPrefix(pkg, "./") {
		return pkg, nil
	}

	pkgs, err := packages.Load(
		&packages.Config{
			BuildFlags: []string{"-toolexec="},
			Dir:        dir,
			Logf:       logf,
			Mode:       packages.NeedName,
		},
		pkg,
	)
	if err != nil {
		return pkg, err
	}

	if len(pkgs) != 1 {
		return pkg, fmt.Errorf("resolution returned %d packages", len(pkgs))
	}

	return pkgs[0].PkgPath, nil
}

func logf(format string, args ...any) {
	log.Tracef(format+"\n", args...)
}
