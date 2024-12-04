// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"bytes"
)

type PackageMayMatchContext struct {
	ImportPath string
	ImportMap  map[string]string
}

func (ctx *PackageMayMatchContext) PackageImports(path string) bool {
	if path == "" {
		return true
	}
	_, ok := ctx.ImportMap[path]
	return ok || path == ctx.ImportPath
}

type FileMayMatchContext struct {
	FileContent []byte
}

func (ctx *FileMayMatchContext) FileContains(content string) bool {
	return bytes.Contains(ctx.FileContent, []byte(content))
}
