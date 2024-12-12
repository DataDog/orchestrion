// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"gopkg.in/yaml.v3"
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

	return may.CantMatch
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

	return may.CantMatch
}

func (p packageName) Matches(ctx context.AspectContext) bool {
	return ctx.Package() == string(p)
}

func (p packageName) Hash(h *fingerprint.Hasher) error {
	return h.Named("import-path", fingerprint.String(p))
}

func init() {
	unmarshalers["import-path"] = func(node *yaml.Node) (Point, error) {
		var name string
		if err := node.Decode(&name); err != nil {
			return nil, err
		}
		return ImportPath(name), nil
	}

	unmarshalers["package-name"] = func(node *yaml.Node) (Point, error) {
		var name string
		if err := node.Decode(&name); err != nil {
			return nil, err
		}
		return PackageName(name), nil
	}
}
