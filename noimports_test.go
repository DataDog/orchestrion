// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInjectImportIntoPackageWithoutImports checks that an aspect can add an import to a package that
// imports nothing, for which the go command passes an importcfg file without any package entry.
func TestInjectImportIntoPackageWithoutImports(t *testing.T) {
	run := runner{dir: t.TempDir()}
	writeFile := func(name, contents string) {
		t.Helper()
		path := filepath.Join(run.dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	writeFile("go.mod", `module example.com/noimports

go 1.25

require github.com/DataDog/orchestrion v0.0.0

replace github.com/DataDog/orchestrion => `+rootDir+"\n")
	writeFile("orchestrion.tool.go", `//go:build tools

package tools

import (
	_ "example.com/noimports/instrumentation"
	_ "github.com/DataDog/orchestrion"
)
`)
	writeFile("instrumentation/instrumentation.go", "package instrumentation\n")
	writeFile("instrumentation/orchestrion.yml", `meta:
  name: Import injection
  description: Adds an import to a package that has none.
aspects:
  - id: inject-import
    join-point:
      all-of:
        - import-path: example.com/noimports/subject
        - function-body:
            function:
              - name: Value
    advice:
      - prepend-statements:
          imports:
            strconv: strconv
          template: Injected = strconv.Itoa(1)
`)
	writeFile("subject/subject.go", "package subject\n\nvar Injected string\n\nfunc Value() int { return 42 }\n")
	writeFile("main.go", `package main

import (
	"log"

	"example.com/noimports/subject"
)

func main() {
	if subject.Value() != 42 || subject.Injected != "1" {
		log.Fatalln("The aspect was not woven")
	}
}
`)
	run.exec(t, "go", "mod", "tidy")

	binary := filepath.Join(t.TempDir(), "app")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	run.exec(t, buildOrchestrion(t), "go", "build", "-o", binary, ".")
	run.exec(t, binary)
}
