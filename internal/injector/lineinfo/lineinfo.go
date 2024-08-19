// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package lineinfo

import (
	"go/ast"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// AnnotateMovedNodes adds `//line` directives to the provided `*dst.File` to adjust source location
// information of each AST node that exists in the original source file to its location there, and
// marks other nodes as originating from `<generated>`.
func AnnotateMovedNodes(
	// The decorator that produced the *dst.File
	decorator *decorator.Decorator,
	// The *dst.File to annotate
	file *dst.File,
	// A function that creates a new *decorator.FileRestorer for the given filename
	newRestorer func(string) *decorator.FileRestorer,
) error {
	canonicalizer := canonicalizationVisitor{}
	// Pre-process the AST to make it closer to the canonical go format, which will allow us to have
	// more accurate "after-printing" line information.
	dst.Walk(&canonicalizer, file)
	if len(canonicalizer.stack) != 0 {
		panic("noempty stack after canonicalizater visit is complete")
	}

	// Restore to an *ast.File so we can obtain the new line information data.
	res := newRestorer(decorator.Filenames[file])
	astFile, err := res.RestoreFile(file)
	if err != nil {
		return err
	}

	// Visit the AST to add `//line` directives where the updated line information no longer matches
	// the original source file's.
	annotator := annotationVisitor{decorator: decorator, restorer: res}
	ast.Walk(&annotator, astFile)
	if len(annotator.stack) != 0 {
		panic("noempty stack after annotation visit is complete")
	}

	return nil
}
