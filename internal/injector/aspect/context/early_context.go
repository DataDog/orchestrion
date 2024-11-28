// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

type EarlyContext struct {
	// ImportPath is the fully qualified import path of the current package
	ImportPath string
	// ImportMap maps package dependencies import paths to their fully-qualified version
	ImportMap map[string]string
}

func (ctx *EarlyContext) PackageImports(importPath string) bool {
	_, ok := ctx.ImportMap[importPath]
	return ok || ctx.ImportPath == importPath
}
