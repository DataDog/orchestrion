// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"context"
	goast "go/ast"
	"go/token"

	"github.com/dave/dst"
)

// RestoreCaptureFunc is a test instrumentation hook. It is invoked once per modified file, right
// after the file has been restored to go/ast and before it is formatted and written to disk.
//
// The arguments are the original file name, the final *dst.File, the decorator's dst-to-ast node
// map (covering nodes that came from the original source files), the restorer's dst-to-ast node
// map (covering the restored output tree), the restored *ast.File, and the file set that carries
// the positions of the original ast nodes.
//
// Production code never sets this hook. The injector behaves identically whether it is present or
// not.
type RestoreCaptureFunc func(filename string, file *dst.File, decoratorNodes map[dst.Node]goast.Node, restorerNodes map[dst.Node]goast.Node, restored *goast.File, fset *token.FileSet)

type restoreCaptureContextKey struct{}

// WithRestoreCapture returns ctx with the provided restore-capture hook attached. It is test
// instrumentation only, and attaching a nil hook leaves the context unchanged.
func WithRestoreCapture(ctx context.Context, hook RestoreCaptureFunc) context.Context {
	if hook == nil {
		return ctx
	}
	return context.WithValue(ctx, restoreCaptureContextKey{}, hook)
}
