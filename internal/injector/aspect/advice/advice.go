// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package advice provides implementations of the injector.Action interface for
// common AST changes.
package advice

import (
	"context"

	"github.com/datadog/orchestrion/internal/injector/code"
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst/dstutil"
)

const pkgPath = "github.com/datadog/orchestrion/internal/injector/aspect/advice"

// Advice is the interface abstracting actual AST changes performed by
// injections.
type Advice interface {
	code.AsCode
	code.ImportAdder

	// Apply performs the necessary AST changes on the supplied node. It returns a
	// boolean indicating whether the node was modified or not (some actions may
	// short-circuit and not do anything; e.g. import injection may be skipped if
	// the import already exists).
	Apply(context.Context, *node.Chain, *dstutil.Cursor) (bool, error)

	// RenderHTML renders and HTML representation of this advice for documentation
	// purposes.
	RenderHTML() string
}
