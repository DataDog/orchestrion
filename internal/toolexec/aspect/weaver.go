// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import "strings"

// variantSeparator introduces the variant annotation Go adds to the
// `$TOOLEXEC_IMPORTPATH` value of packages it builds more than once, as
// implemented by `(*load.Package).Desc` in `cmd/go`. Module import paths can
// contain neither spaces nor square brackets, so this cannot appear in the
// import path of a package Orchestrion can process (it requires module-aware
// builds, as it resolves its configuration from the module's `go.mod` file).
const variantSeparator = " ["

// testMainSuffix is the suffix Go appends to a package's import path to name the
// generated main package of its test binary.
const testMainSuffix = ".test"

// Weaver applies aspects to the Go toolchain commands used to build a package.
// It is created from the `$TOOLEXEC_IMPORTPATH` value Go provides for those
// commands, using [NewWeaver].
type Weaver struct {
	// ImportPath is the import path of the package being built, without the
	// variant annotation Go adds to `$TOOLEXEC_IMPORTPATH`; so that aspects apply
	// to every variant of a package exactly as they do to its ordinary build.
	ImportPath string
	// Variant identifies why Go is building a variant of [Weaver.ImportPath], if
	// it is. It is the import path of the test binary a test variant is built for
	// (e.g, `example.com/pkg.test`), or that of the main package a
	// profile-guided optimization variant is specialized for. It is blank when Go
	// builds the package's ordinary variant.
	Variant string
}

// NewWeaver returns the [Weaver] for the package designated by the provided
// `$TOOLEXEC_IMPORTPATH` value.
//
// Go annotates that value for packages it builds more than once, so that
// `example.com/pkg` becomes `example.com/pkg [example.com/pkg.test]` when it is
// re-built with its in-package test files, for example. Such annotations must
// not be mistaken for a part of the package's import path, as this would result
// in aspects not being applied to those variants at all.
func NewWeaver(toolexecImportPath string) Weaver {
	importPath, variant, hasVariant := strings.Cut(toolexecImportPath, variantSeparator)
	if !hasVariant {
		return Weaver{ImportPath: toolexecImportPath}
	}
	return Weaver{ImportPath: importPath, Variant: strings.TrimSuffix(variant, "]")}
}

// isTestMain returns true if the package being built is a test binary's
// generated main package (e.g, `example.com/pkg.test`), as opposed to any of the
// packages that test binary is composed of. As a package can legitimately be
// named `<something>.test`, callers should also verify the compilation's inputs
// include Go's generated test-main source (see [proxy.CompileCommand.TestMain])
// unless they cannot, which is the case when the source is served from Go's
// build cache under a content-addressed file name.
func (w Weaver) isTestMain() bool {
	// Go does not build variants of the generated main package of a test binary.
	return w.Variant == "" && strings.HasSuffix(w.ImportPath, testMainSuffix)
}

// packageUnderTest returns the import path of the package a test binary's
// generated main package tests. The receiver must satisfy [Weaver.isTestMain].
func (w Weaver) packageUnderTest() string {
	return strings.TrimSuffix(w.ImportPath, testMainSuffix)
}
