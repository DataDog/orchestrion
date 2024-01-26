// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

type (
	// ReferenceKind denotes the style of a reference, which influences compilation and linking requirements.
	ReferenceKind bool

	// ReferenceMap associates import paths to ReferenceKind values.
	ReferenceMap map[string]ReferenceKind
)

const (
	// ImportStatement references must be made available to the compiler via the provided `importcfg`.
	ImportStatement ReferenceKind = true
	// RelocationTarget references must be made available to the linker, and must be referenced (directly or not) by the main package.
	RelocationTarget ReferenceKind = false
)
