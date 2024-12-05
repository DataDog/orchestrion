// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package may

import (
	"bytes"
)

type PackageContext struct {
	ImportPath string
	ImportMap  map[string]string
	TestMain   bool
}

func (ctx *PackageContext) PackageImports(path string) MatchType {
	if path == "" {
		return Unknown
	}
	_, ok := ctx.ImportMap[path]
	if ok || path == ctx.ImportPath {
		return Match
	}

	return CantMatch
}

type FileMayMatchContext struct {
	FileContent []byte
}

func (ctx *FileMayMatchContext) FileContains(content string) MatchType {
	if bytes.Contains(ctx.FileContent, []byte(content)) {
		return Match
	}

	return CantMatch
}
