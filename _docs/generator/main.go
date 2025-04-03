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

	// Document the V1 integrations
	gen := Generator{
		Dir:          filepath.Join(thisFile, "..", "..", "content", "docs", "dd-trace-go", "v1"),
		ConfigSource: filepath.Join(thisFile, "..", "..", "..", "instrument"),
		Validate:     true,
		CommonPrefix: "gopkg.in/DataDog/dd-trace-go.v1/",
	}
	if err := gen.Generate(); err != nil {
		log.Fatalln(err)
	}

	// Document the V2 integrations
	gen = Generator{
		Dir:          filepath.Join(thisFile, "..", "..", "content", "docs", "dd-trace-go", "v2"),
		ConfigSource: filepath.Join(thisFile, "..", ".."),
		Validate:     false, // Currently one aspect is not valid in V2 (rueidis)
		CommonPrefix: "github.com/DataDog/dd-trace-go/",
		TrimPrefix:   "v2/",
		TrimSuffix:   "/v2",
	}
	if err := gen.Generate(); err != nil {
		log.Fatalln(err)
	}

	// Document the aspects schema
	if err := documentSchema(filepath.Join(thisFile, "..", "..", "content", "contributing", "aspects")); err != nil {
		log.Fatalln(err)
	}
}
