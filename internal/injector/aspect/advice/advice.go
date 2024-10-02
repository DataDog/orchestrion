// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package advice provides implementations of the injector.Action interface for
// common AST changes.
package advice

import (
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/jennifer/jen"
)

const pkgPath = "github.com/DataDog/orchestrion/internal/injector/aspect/advice"

// Advice is the interface abstracting actual AST changes performed by
// injections.
type Advice interface {
	// AsCode produces a jen.Code representation of the receiver.
	AsCode() jen.Code

	// AddedImports returns the list of import paths the receiver may introduce in
	// modified code.
	AddedImports() []string

	// Apply performs the necessary AST changes on the supplied node. It returns a
	// boolean indicating whether the node was modified or not (some actions may
	// short-circuit and not do anything; e.g. import injection may be skipped if
	// the import already exists).
	Apply(context.AdviceContext) (bool, error)
}
