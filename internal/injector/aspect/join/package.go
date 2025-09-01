// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"errors"
	"path/filepath"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/goccy/go-yaml/ast"
)

type importPath string

func ImportPath(name string) importPath {
	return importPath(name)
}

func (p importPath) ImpliesImported() []string {
	return []string{string(p)} // Technically the current package in this instance
}

func (p importPath) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	if ctx.ImportPath == string(p) {
		return may.Match
	}

	return may.NeverMatch
}

func (importPath) FileMayMatch(_ *may.FileContext) may.MatchType {
	return may.Unknown
}

func (p importPath) Matches(ctx context.AspectContext) bool {
	return ctx.ImportPath() == string(p)
}

func (p importPath) Hash(h *fingerprint.Hasher) error {
	return h.Named("import-path", fingerprint.String(p))
}

type packageName string

func PackageName(name string) packageName {
	return packageName(name)
}

func (packageName) ImpliesImported() []string {
	return nil // Can't assume anything here...
}

func (packageName) PackageMayMatch(_ *may.PackageContext) may.MatchType {
	return may.Unknown
}

func (p packageName) FileMayMatch(ctx *may.FileContext) may.MatchType {
	if ctx.PackageName == string(p) {
		return may.Match
	}

	return may.NeverMatch
}

func (p packageName) Matches(ctx context.AspectContext) bool {
	return ctx.Package() == string(p)
}

func (p packageName) Hash(h *fingerprint.Hasher) error {
	return h.Named("import-path", fingerprint.String(p))
}

type packageFilter struct {
	root    bool   // true if targeting the root module only, false for global matching
	pattern string // glob pattern for import path matching (uses filepath.Match)
}

// PackageFilter creates a package filter join point that matches import paths using glob patterns.
//
// If root is true, only matches packages within the current Go module and applies the pattern
// to relative paths within the module.
// If root is false, matches packages from any module
// using the full import path.
//
// Examples:
//
//	PackageFilter(true, "internal/*")  - matches internal packages in root module only
//	PackageFilter(false, "*/internal/*") - matches internal packages in any module
//	PackageFilter(false, "github.com/myorg/*") - matches any package in myorg
func PackageFilter(root bool, pattern string) packageFilter {
	return packageFilter{root: root, pattern: pattern}
}

func (_ packageFilter) ImpliesImported() []string {
	return nil // Cannot determine specific imports from a pattern
}

func (pf packageFilter) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	if pf.matchesPattern(ctx.ImportPath) {
		return may.Match
	}
	return may.NeverMatch
}

func (_ packageFilter) FileMayMatch(_ *may.FileContext) may.MatchType {
	return may.Unknown
}

func (pf packageFilter) Matches(ctx context.AspectContext) bool {
	return pf.matchesPattern(ctx.ImportPath())
}

func (pf packageFilter) Hash(h *fingerprint.Hasher) error {
	return h.Named("package-filter",
		fingerprint.Bool(pf.root),
		fingerprint.String(pf.pattern),
	)
}

func (pf packageFilter) matchesPattern(importPath string) bool {
	pathToMatch := importPath
	if pf.root {
		// Root module filtering - only match packages in the root module
		rootPath, err := goenv.RootModulePath(gocontext.Background())
		if err != nil {
			return false // If we can't determine root module, assume no match
		}

		// Check if the import path belongs to the root module
		if !isInRootModule(importPath) {
			return false
		}

		// Use relative path within the root module for pattern matching
		pathToMatch = getRelativePathInModule(importPath, rootPath)
	}

	if pf.pattern == "*" {
		return true
	}

	matched, err := filepath.Match(pf.pattern, pathToMatch)
	if err != nil {
		return false
	}
	return matched
}

// isInRootModule checks if the given import path belongs to the root module.
func isInRootModule(importPath string) bool {
	rootPath, err := goenv.RootModulePath(gocontext.Background())
	if err != nil {
		return false // If we can't determine, assume it doesn't match
	}

	return filepath.HasPrefix(importPath, rootPath)
}

// getRelativePathInModule returns the relative path of an import path within its module.
func getRelativePathInModule(importPath string, rootModulePath string) string {
	if importPath == rootModulePath {
		return "."
	}

	if filepath.HasPrefix(importPath, rootModulePath) {
		relative := importPath[len(rootModulePath):]
		if len(relative) > 0 && relative[0] == '/' {
			relative = relative[1:]
		}
		return relative
	}

	return importPath
}

func init() {
	unmarshalers["import-path"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var name string
		if err := yaml.NodeToValueContext(ctx, node, &name); err != nil {
			return nil, err
		}
		return ImportPath(name), nil
	}

	unmarshalers["package-name"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var name string
		if err := yaml.NodeToValueContext(ctx, node, &name); err != nil {
			return nil, err
		}
		return PackageName(name), nil
	}

	unmarshalers["package-filter"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var pattern string
		if err := yaml.NodeToValueContext(ctx, node, &pattern); err == nil {
			return PackageFilter(false, pattern), nil
		}
		var config struct {
			Root    bool   `yaml:"root"`
			Pattern string `yaml:"pattern"`
		}
		if err := yaml.NodeToValueContext(ctx, node, &config); err != nil {
			return nil, err
		}

		if config.Pattern == "" {
			return nil, errors.New("package-filter requires a 'pattern' field")
		}

		return PackageFilter(config.Root, config.Pattern), nil
	}
}
