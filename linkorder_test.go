// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLinkDependencyImportsAreDeterministic checks that the blank imports orchestrion adds to main
// packages for link-time dependencies do not depend on the order in which it resolves them.
//
// In each copy of the fixture, resolving `s` brings `p` and `q` into the build; `p` has a link-time
// dependency on `i2`, which imports `x`; and `q` has one on `i3`, which has a link-time dependency on
// `x`. Whether `x` gets a blank import depends on whether `i2` or `i3` is resolved first, which
// previously happened in the minority order in about 1 in 8 compiles; with 4 copies and 10 main
// packages, the previous code fails all but about 0.5% of runs.
func TestLinkDependencyImportsAreDeterministic(t *testing.T) {
	const copies = 4

	run := runner{dir: t.TempDir()}
	writeFile := func(name, contents string) {
		t.Helper()
		path := filepath.Join(run.dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	writeFile("go.mod", `module example.com/linkorder

go 1.25

require github.com/DataDog/orchestrion v0.0.0

replace github.com/DataDog/orchestrion => `+rootDir+"\n")
	writeFile("orchestrion.tool.go", `//go:build tools

package tools

import (
	_ "example.com/linkorder/instrumentation"
	_ "github.com/DataDog/orchestrion"
)
`)
	writeFile("instrumentation/instrumentation.go", "package instrumentation\n")

	// linkAspect injects into from's Fn a declaration linked to to's Fn, which records a link-time
	// dependency of from on to.
	linkAspect := func(from, to string) string {
		return fmt.Sprintf(`  - id: %[1]s-links-%[2]s
    join-point:
      all-of:
        - import-path: example.com/linkorder/%[1]s
        - function-body:
            function:
              - name: Fn
    advice:
      - inject-declarations:
          links:
            - example.com/linkorder/%[2]s
          template: |-
            //go:linkname __linkorder_%[2]s example.com/linkorder/%[2]s.Fn
            func __linkorder_%[2]s()
`, from, to)
	}
	aspects := []string{"meta:\n  name: Link order\n  description: Link-time dependencies resolved in varying order.\naspects:\n"}
	writeFile("root/root.go", "package root\n\nfunc Fn() {}\n")
	for i := range copies {
		pkg := func(name string) string { return fmt.Sprintf("%s%d", name, i) }
		aspects = append(aspects,
			linkAspect("root", pkg("s")),
			linkAspect(pkg("p"), pkg("i2")),
			linkAspect(pkg("q"), pkg("i3")),
			linkAspect(pkg("i3"), pkg("x")),
		)
		for _, name := range []string{pkg("p"), pkg("q"), pkg("i3"), pkg("x")} {
			writeFile(name+"/"+name+".go", fmt.Sprintf("package %s\n\nfunc Fn() {}\n", name))
		}
		writeFile(pkg("s")+"/"+pkg("s")+".go", fmt.Sprintf(`package %[1]s

import (
	"example.com/linkorder/%[2]s"
	"example.com/linkorder/%[3]s"
)

func Fn() { %[2]s.Fn(); %[3]s.Fn() }
`, pkg("s"), pkg("p"), pkg("q")))
		writeFile(pkg("i2")+"/"+pkg("i2")+".go", fmt.Sprintf(`package %[1]s

import "example.com/linkorder/%[2]s"

func Fn() { %[2]s.Fn() }
`, pkg("i2"), pkg("x")))
	}
	writeFile("instrumentation/orchestrion.yml", strings.Join(aspects, ""))
	// Each main package is compiled by its own toolexec process, which resolves its link-time
	// dependencies independently of the others.
	const mains = 10
	for i := range mains {
		writeFile(fmt.Sprintf("cmd/main%d/main.go", i), "package main\n\nimport \"example.com/linkorder/root\"\n\nfunc main() { root.Fn() }\n")
	}
	run.exec(t, "go", "mod", "tidy")

	imports := buildLinkDepsImports(t, run.dir, buildOrchestrion(t))
	require.Len(t, imports, mains)
	for i := range copies {
		require.Contains(t, imports[0], fmt.Sprintf("example.com/linkorder/s%d", i))
	}
	for _, content := range imports[1:] {
		require.Equal(t, imports[0], content, "main packages got different blank imports")
	}
}

// buildLinkDepsImports builds the main packages of the module in dir with orchestrion, and returns the
// content of the synthetic link_deps_imports.go files it added to them.
func buildLinkDepsImports(t *testing.T, dir string, orchestrion string) []string {
	t.Helper()

	cmd := exec.Command(orchestrion, "go", "build", "-work", "./cmd/...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE="+t.TempDir())
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	require.NoError(t, cmd.Run(), "build failed:\n%s", &output)

	var work string
	for _, line := range strings.Split(output.String(), "\n") {
		if strings.HasPrefix(line, "WORK=") {
			work = strings.TrimSpace(strings.TrimPrefix(line, "WORK="))
			break
		}
	}
	require.NotEmpty(t, work, "no WORK directory in the build output:\n%s", &output)
	defer os.RemoveAll(work)

	files, err := filepath.Glob(filepath.Join(work, "*", "orchestrion", "src", "synthetic", "link_deps_imports.go"))
	require.NoError(t, err)
	contents := make([]string, len(files))
	for i, file := range files {
		content, err := os.ReadFile(file)
		require.NoError(t, err)
		contents[i] = string(content)
	}
	return contents
}
