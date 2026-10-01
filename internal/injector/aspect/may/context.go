// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package may

import (
	"bytes"
	"index/suffixarray"
	"sync"
	"sync/atomic"
)

// PackageContext is the context for a package to be matched.
type PackageContext struct {
	// ImportPath is the import path of the package in its module.
	ImportPath string

	// ImportMap is the map of import paths to their respective package archives
	ImportMap map[string]string

	// TestMain is true if the package is a test package.
	TestMain bool

	// WorkDir is the directory of the build, whose module is the root module (e.g, for
	// `package-filter` join points with `root: true`). If blank, the current working directory is
	// used.
	WorkDir string
}

func (ctx *PackageContext) PackageImports(path string) MatchType {
	if path == "" {
		return Unknown
	}
	_, ok := ctx.ImportMap[path]
	if ok || path == ctx.ImportPath {
		return Match
	}

	return NeverMatch
}

// maxLinearLookups is the number of [FileContext.FileContains] lookups answered by scanning the
// file's content, after which an index of the content is built to answer subsequent lookups.
// Building the index costs about as much as a hundred scans, and files typically only get a
// handful of lookups; so this keeps the common case cheap while bounding the worst case.
const maxLinearLookups = 64

// FileContext is the context for a file to be matched.
type FileContext struct {
	// FileContent is the content of the file to be matched.
	FileContent []byte

	// PackageName is the name of the package given as seen in `package main` for example.
	PackageName string

	lookups atomic.Int32
	once    sync.Once
	index   *suffixarray.Index
}

func (ctx *FileContext) FileContains(content string) MatchType {
	if content == "" {
		// The empty string is never found (this is how [suffixarray.Index.Lookup] behaves).
		return NeverMatch
	}

	var found bool
	if ctx.lookups.Add(1) <= maxLinearLookups {
		found = bytes.Contains(ctx.FileContent, []byte(content))
	} else {
		ctx.once.Do(func() {
			ctx.index = suffixarray.New(ctx.FileContent)
		})
		found = len(ctx.index.Lookup([]byte(content), 1)) > 0
	}

	if found {
		return Match
	}

	return NeverMatch
}
