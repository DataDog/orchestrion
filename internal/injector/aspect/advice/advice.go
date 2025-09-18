// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package advice provides implementations of the injector.Action interface for
// common AST changes.
package advice

import (
	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
)

// Advice is the interface abstracting actual AST changes performed by
// injections.
type Advice interface {
	// AddedImports returns the list of import paths the receiver may introduce in
	// modified code.
	AddedImports() []string

	// Apply performs the necessary AST changes on the supplied node. It returns a
	// boolean indicating whether the node was modified or not (some actions may
	// short-circuit and not do anything; e.g. import injection may be skipped if
	// the import already exists).
	Apply(context.AdviceContext) (bool, error)

	fingerprint.Hashable
}

// OrderableAdvice is an optional interface that advice can implement to provide
// ordering information for deterministic execution order.
type OrderableAdvice interface {
	Advice

	// Order returns the execution order within the namespace.
	// Lower values execute first.
	Order() int

	// Namespace returns the logical grouping namespace for this advice.
	Namespace() string
}
