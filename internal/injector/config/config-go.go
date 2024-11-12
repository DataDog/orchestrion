// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"golang.org/x/tools/go/packages"
)

const OrchestrionToolGo = "orchestrion.tool.go"

var ErrInvalidGoPackage = errors.New("no .go files in package")

// loadGoPackage loads configuration from the specified go package.
func (l *Loader) loadGoPackage(pkg *packages.Package) (*configGo, error) {
	root := packageRoot(pkg)
	if root == "" {
		// This might be explained by a package-level loading error... We only check
		// here because "all .go files are excluded by build constraints" is one
		// such error that we typically ignore.
		if pkg.Errors != nil {
			var err error
			for _, e := range pkg.Errors {
				err = errors.Join(err, e)
			}
			return nil, fmt.Errorf("in %q: %w", pkg.ID, err)
		}

		return nil, fmt.Errorf("%q: %w", pkg.PkgPath, ErrInvalidGoPackage)
	}

	toolFile := filepath.Join(root, OrchestrionToolGo)
	imports, err := l.loadGoFile(toolFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	ymlCfg, err := l.loadYMLFile(root, OrchestrionYML)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return &configGo{imports: imports, yaml: ymlCfg, pkgPath: pkg.PkgPath}, nil
}

// loadGoFile loads configuration from the specified go file. Returns nil if the
// file had already been loaded previously, or if the configuration is empty.
func (l *Loader) loadGoFile(filename string) ([]Config, error) {
	if !l.markLoaded(filename) {
		return nil, nil
	}

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

	pkgs, err := l.packages(imports...)
	if err != nil {
		return nil, fmt.Errorf("imports from %q (%q): %w", filename, imports, err)
	}

	cfgs := make([]Config, 0, len(pkgs))
	for _, pkg := range pkgs {
		cfg, err := l.loadGoPackage(pkg)
		if err != nil {
			return nil, fmt.Errorf("in %q imported from %q: %w", pkg.PkgPath, filename, err)
		}
		if cfg == nil {
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
	var res []*aspect.Aspect

	for _, imp := range c.imports {
		res = append(res, imp.Aspects()...)
	}

	if c.yaml != nil {
		res = append(res, c.yaml.Aspects()...)
	}

	return res
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
