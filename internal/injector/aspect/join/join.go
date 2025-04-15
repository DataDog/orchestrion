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

// TypeName struct and methods moved to internal/injector/typed/typename.go
