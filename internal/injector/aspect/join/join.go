// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package join provides implementations of the InjectionPoint interface for
// common injection points.
package join

import (
	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
)

// Point is the interface that abstracts selection of nodes where to inject
// code.
type Point interface {
	// ImpliesImported returns a list of import paths that are known to already be
	// imported if the join point matches.
	ImpliesImported() []string

	// PackageMayMatch determines whether the join point may match the given package data from the importcfg file.
	PackageMayMatch(ctx *may.PackageContext) may.MatchType

	// FileMayMatch determines whether the join point may match the given file raw content
	FileMayMatch(ctx *may.FileContext) may.MatchType

	// Matches determines whether the injection should be performed on the given
	// node or not. The node's ancestry is also provided to allow Point to make
	// decisions based on parent nodes.
	Matches(ctx context.AspectContext) bool

	fingerprint.Hashable
}

// NeedsTypesMap reports whether any of the given join points require the
// types.Info.Types map to be populated. This is only needed for *implements
// join points (resultImplements, finalResultImplements, argumentImplements).
// When false, the type checker can skip populating the Types map, saving
// per-expression map insertions.
func NeedsTypesMap(points []Point) bool {
	for _, p := range points {
		if needsTypesMap(p) {
			return true
		}
	}
	return false
}

func needsTypesMap(p Point) bool {
	switch v := p.(type) {
	case allOf:
		for _, child := range v {
			if needsTypesMap(child) {
				return true
			}
		}
	case oneOf:
		for _, child := range v {
			if needsTypesMap(child) {
				return true
			}
		}
	case *not:
		return needsTypesMap(v.JoinPoint)
	case *functionBody:
		return needsTypesMap(v.Function)
	case *functionDeclaration:
		for _, opt := range v.Options {
			switch opt.(type) {
			case *resultImplements, *finalResultImplements, *argumentImplements:
				return true
			}
		}
	}
	return false
}
