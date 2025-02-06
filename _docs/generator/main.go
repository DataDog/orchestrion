// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package main is a generator that renders the documentation pages for aspects
// that are baked into the `dd-trace-go` package.
package main

import (
	"log"
	"path/filepath"
	"runtime"
)

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	gen := Generator{
		Dir:          filepath.Join(thisFile, "..", "..", "content", "docs", "dd-trace-go", "integrations"),
		ConfigSource: filepath.Join(thisFile, "..", "..", "..", "instrument"),
	}
	if err := gen.Generate(); err != nil {
		log.Fatalln(err)
	}
}
