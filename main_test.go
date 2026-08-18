// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/cover"
)

func TestSyntheticLinkDependencyUsesTestVariant(t *testing.T) {
	run := runner{dir: t.TempDir()}
	writeFile := func(name, contents string) {
		t.Helper()
		path := filepath.Join(run.dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	writeFile("go.mod", `module example.com/testvariant

go 1.25

require github.com/DataDog/orchestrion v0.0.0

replace github.com/DataDog/orchestrion => `+rootDir+"\n")
	writeFile("orchestrion.tool.go", `//go:build tools

package tools

import (
	_ "example.com/testvariant/instrumentation"
	_ "github.com/DataDog/orchestrion"
)
`)
	writeFile("instrumentation/instrumentation.go", "package instrumentation\n")
	writeFile("instrumentation/orchestrion.yml", `meta:
  name: Test variant link dependency
  description: Adds a synthetic dependency that imports the package under test.
aspects:
  - id: synthetic-test-variant
    join-point:
      all-of:
        - import-path: example.com/testvariant/subject
        - function-body:
            function:
              - name: Value
    advice:
      - inject-declarations:
          links:
            - example.com/testvariant/root
            - example.com/testvariant/dep/internal/root
          template: |-
            //go:linkname __orchestrionRootValue example.com/testvariant/root.Value
            func __orchestrionRootValue() int

            //go:linkname __orchestrionInternalRootValue example.com/testvariant/dep/internal/root.Value
            func __orchestrionInternalRootValue() int
`)
	writeFile("subject/subject.go", `package subject

import (
	"example.com/testvariant/leaf"
	"example.com/testvariant/notests"
)

func Value() int { return 40 + leaf.Value() + notests.Value() }
`)
	writeFile("subject/subject_test.go", `package subject

import "testing"

func TestValue(t *testing.T) {
	for range 3 {
		if got := Value(); got != 42 {
			t.Fatalf("Value() = %d, want 42", got)
		}
	}
}
`)
	writeFile("leaf/leaf.go", "package leaf\n\nfunc Value() int { return 1 }\n")
	writeFile("notests/notests.go", "package notests\n\nfunc Value() int { return 1 }\n")
	writeFile("leaf/leaf_test.go", `package leaf

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != 1 {
		t.Fatalf("Value() = %d, want 1", got)
	}
}
`)
	writeFile("root/root.go", `package root

import (
	"example.com/testvariant/leaf"
	"example.com/testvariant/subject"
)

func Value() int { return subject.Value() + leaf.Value() - 1 }
`)
	writeFile("dep/internal/root/root.go", `package root

import "example.com/testvariant/subject"

func Value() int { return subject.Value() }
`)

	orchestrion := buildOrchestrion(t)
	run.exec(t, orchestrion, "go", "test", "-a", "./subject")
	run.exec(t, orchestrion, "go", "test", "-a", "-cover", "-coverpkg=./...", "./subject")

	// A multi-package coverage run compiles root against the ordinary subject archive before
	// Orchestrion injects it into the subject test binary, whose subject archive has coverage.
	run.exec(t, orchestrion, "go", "test", "-a", "-coverprofile="+filepath.Join(run.dir, "coverage.out"), "./root", "./subject")

	// Without an explicit -coverpkg, Go covers each package that has tests only in its own test
	// binary, while covering command-line packages without tests in their ordinary form. The nested
	// load for the subject test binary must therefore leave leaf uninstrumented but instrument
	// notests, matching the archives against which subject was compiled.
	run.exec(t, orchestrion, "go", "test", "-a", "-coverprofile="+filepath.Join(run.dir, "coverage-all.out"), "./...")

	// Coverage mode implied by -race (rather than an explicit -covermode) must remain consistent
	// between the ordinary archive and each per-binary-scoped test-variant archive. Repeated calls
	// to subject.Value distinguish atomic counters from set counters instead of only checking that
	// the build succeeds.
	t.Run("RaceCoverMode", func(t *testing.T) {
		t.Setenv("GOFLAGS", "")
		profile := filepath.Join(run.dir, "coverage-race.out")
		run.exec(t, orchestrion, "go", "test", "-a", "-race", "-coverprofile="+profile, "./...")
		requireCoverageCountAtLeast(t, profile, "example.com/testvariant/subject/subject.go", "atomic", 3)
	})

	// Implied build modes supplied through GOFLAGS must reach the nested scoped rebuilds too.
	t.Run("RaceCoverModeFromGOFLAGS", func(t *testing.T) {
		t.Setenv("GOFLAGS", "-race")
		profile := filepath.Join(run.dir, "coverage-goflags-race.out")
		run.exec(t, orchestrion, "go", "test", "-a", "-coverprofile="+profile, "./...")
		requireCoverageCountAtLeast(t, profile, "example.com/testvariant/subject/subject.go", "atomic", 3)
	})

	// Value-less test flags must not consume the package patterns that follow them, as coverage is
	// otherwise applied to the wrong packages in nested loads.
	run.exec(t, orchestrion, "go", "test", "-a", "-coverprofile="+filepath.Join(run.dir, "coverage-v.out"), "-v", "./subject", "./root")

	// The nested test-variant load must preserve an overlay supplied to the outer Go command.
	writeFile("subject/subject.go", "package subject\n\nfunc Value() int { return missing }\n")
	writeFile("overlay/subject.go", `package subject

import (
	"example.com/testvariant/leaf"
	"example.com/testvariant/notests"
)

func Value() int { return 40 + leaf.Value() + notests.Value() }
`)
	writeFile("overlay.json", `{"Replace":{"subject/subject.go":"overlay/subject.go"}}`)
	run.exec(t, orchestrion, "go", "test", "-a", "-overlay=overlay.json", "./subject")
}

func TestSyntheticLinkDependencyWithExternalTests(t *testing.T) {
	run := runner{dir: t.TempDir()}
	writeFile := func(name, contents string) {
		t.Helper()
		path := filepath.Join(run.dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	writeFile("go.mod", `module example.com/externaltestvariant

go 1.25

require github.com/DataDog/orchestrion v0.0.0

replace github.com/DataDog/orchestrion => `+rootDir+"\n")
	writeFile("orchestrion.tool.go", `//go:build tools

package tools

import (
	_ "example.com/externaltestvariant/instrumentation"
	_ "github.com/DataDog/orchestrion"
)
`)
	writeFile("instrumentation/instrumentation.go", "package instrumentation\n")
	writeFile("instrumentation/orchestrion.yml", `meta:
  name: External test variant link dependency
  description: Exercises nested cached test-main inputs.
aspects:
  - id: synthetic-external-test-variant
    join-point:
      all-of:
        - import-path: example.com/externaltestvariant/subject
        - function-body:
            function:
              - name: Value
    advice:
      - inject-declarations:
          links:
            - example.com/externaltestvariant/root
          template: |-
            //go:linkname __orchestrionRootValue example.com/externaltestvariant/root.Value
            func __orchestrionRootValue() int
`)
	writeFile("subject/subject.go", "package subject\n\nfunc Value() int { return 42 }\n")
	writeFile("subject/subject_test.go", `package subject_test

import (
	"testing"
	"example.com/externaltestvariant/subject"
)

func TestValue(t *testing.T) {
	if got := subject.Value(); got != 42 {
		t.Fatalf("Value() = %d, want 42", got)
	}
}
`)
	writeFile("middle/middle.go", `package middle

import "example.com/externaltestvariant/subject"

func Value() int { return subject.Value() }
`)
	writeFile("root/root.go", `package root

import "example.com/externaltestvariant/middle"

func Value() int { return middle.Value() }
`)

	orchestrion := buildOrchestrion(t)
	run.exec(t, orchestrion, "go", "test", "-a", "./subject")
	run.exec(t, orchestrion, "go", "test", "-a", "-coverprofile="+filepath.Join(run.dir, "coverage.out"), "./subject")
	run.exec(t, orchestrion, "go", "test", "-a", "-cover", "-coverpkg=./...", "./subject")
}

func TestSyntheticImportDependencyWithExternalTests(t *testing.T) {
	run := runner{dir: t.TempDir()}
	writeFile := func(name, contents string) {
		t.Helper()
		path := filepath.Join(run.dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	writeFile("go.mod", `module example.com/reversetestvariant

go 1.25

require github.com/DataDog/orchestrion v0.0.0

replace github.com/DataDog/orchestrion => `+rootDir+"\n")
	writeFile("orchestrion.tool.go", `//go:build tools

package tools

import (
	_ "example.com/reversetestvariant/instrumentation"
	_ "github.com/DataDog/orchestrion"
)
`)
	writeFile("instrumentation/instrumentation.go", "package instrumentation\n")
	writeFile("instrumentation/orchestrion.yml", `meta:
  name: Reversed external test variant import
  description: Injects an import of the covered package under test.
aspects:
  - id: reversed-external-test-variant
    join-point:
      all-of:
        - import-path: example.com/reversetestvariant/importer
        - function-body:
            function:
              - name: Value
    advice:
      - inject-declarations:
          imports:
            subject: example.com/reversetestvariant/subject
          template: |-
            func init() {
              subject.Instrumented++
            }
`)
	writeFile("subject/subject.go", `package subject

var Instrumented int

func Value() int { return 42 }
`)
	writeFile("subject/subject_test.go", `package subject_test

import (
	"testing"

	"example.com/reversetestvariant/helper"
	"example.com/reversetestvariant/subject"
)

func TestValue(t *testing.T) {
	if got := helper.Value(); got != 42 {
		t.Fatalf("helper.Value() = %d, want 42", got)
	}
	if got := subject.Instrumented; got != 1 {
		t.Fatalf("subject.Instrumented = %d, want 1", got)
	}
}
`)
	writeFile("helper/helper.go", `package helper

import "example.com/reversetestvariant/importer"

func Value() int { return importer.Value() }
`)
	writeFile("importer/importer.go", `package importer

import "fmt"

func Value() int {
	if fmt.Sprint(42) == "42" {
		return 42
	}
	return 0
}
`)
	orchestrion := buildOrchestrion(t)
	sharedCache := t.TempDir()
	run.execWithCache(t, sharedCache, orchestrion, "go", "test", "./subject")
	run.execWithCache(t, sharedCache, orchestrion, "go", "test", "-coverprofile="+filepath.Join(run.dir, "coverage.out"), "./...")
	// Reverse variants must not poison ordinary action IDs, and their flavored
	// action IDs must remain reusable by subsequent covered builds.
	run.execWithCache(t, sharedCache, orchestrion, "go", "test", "./subject")
	run.execWithCache(t, sharedCache, orchestrion, "go", "test", "-coverprofile="+filepath.Join(run.dir, "coverage-cached.out"), "./...")
	run.exec(t, orchestrion, "go", "test", "-cover", "-coverpkg=./...", "./subject")

	// In-package tests make the package under test part of the importer up-set.
	// Rebuilding that cycle is unsafe, so preserve the existing explicit failure
	// instead of allowing a linker fingerprint mismatch.
	writeFile("subject/subject_test.go", `package subject

import (
	"testing"

	"example.com/reversetestvariant/helper"
)

func TestValue(t *testing.T) {
	if got := helper.Value(); got != 42 {
		t.Fatalf("helper.Value() = %d, want 42", got)
	}
	if got := Instrumented; got != 1 {
		t.Fatalf("Instrumented = %d, want 1", got)
	}
}
`)
	output := run.execError(t, orchestrion, "go", "test", "-a", "-coverprofile="+filepath.Join(run.dir, "same-package-coverage.out"), "./...")
	require.Contains(t, output, "cannot safely use the variant")
	require.NotContains(t, output, "fingerprint mismatch")
}

func TestAspectsApplyToTestVariants(t *testing.T) {
	run := runner{dir: t.TempDir()}
	writeFile := func(name, contents string) {
		t.Helper()
		path := filepath.Join(run.dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	writeFile("go.mod", `module example.com/testvariantaspect

go 1.25

require github.com/DataDog/orchestrion v0.0.0

replace github.com/DataDog/orchestrion => `+rootDir+"\n")
	writeFile("orchestrion.tool.go", `//go:build tools

package tools

import (
	_ "example.com/testvariantaspect/instrumentation"
	_ "github.com/DataDog/orchestrion"
)
`)
	writeFile("instrumentation/instrumentation.go", "package instrumentation\n")
	writeFile("instrumentation/orchestrion.yml", `meta:
  name: Test variant aspects
  description: Records that the packages a test binary is composed of were instrumented.
aspects:
  - id: instrument-subject
    join-point:
      all-of:
        - import-path: example.com/testvariantaspect/subject
        - function-body:
            function:
              - name: Value
    advice:
      - prepend-statements:
          template: |-
            Instrumented = true
  - id: instrument-helper
    join-point:
      all-of:
        - import-path: example.com/testvariantaspect/helper
        - function-body:
            function:
              - name: Doubled
    advice:
      - prepend-statements:
          template: |-
            Instrumented = true
  - id: instrument-external-test
    join-point:
      all-of:
        - import-path: example.com/testvariantaspect/subject_test
        - function-body:
            function:
              - name: value
    advice:
      - prepend-statements:
          template: |-
            instrumented = true
`)
	writeFile("subject/subject.go", `package subject

// Instrumented is set by the instrumentation aspect when Value is called.
var Instrumented bool

func Value() int { return 42 }
`)
	// Go rebuilds the importers of the package under test against its test variant, and identifies those
	// with an annotated $TOOLEXEC_IMPORTPATH value as well.
	writeFile("helper/helper.go", `package helper

import "example.com/testvariantaspect/subject"

// Instrumented is set by the instrumentation aspect when Doubled is called.
var Instrumented bool

func Doubled() int { return 2 * subject.Value() }
`)
	// Go builds the package under test again together with its in-package test files, which it identifies
	// with an annotated $TOOLEXEC_IMPORTPATH value. Aspects must apply to that variant, too.
	writeFile("subject/subject_test.go", `package subject

import "testing"

func TestInPackage(t *testing.T) {
	if got := Value(); got != 42 {
		t.Fatalf("Value() = %d, want 42", got)
	}
	if !Instrumented {
		t.Fatal("aspects were not applied to the in-package test variant of the package under test")
	}
}
`)
	writeFile("subject/external_test.go", `package subject_test

import (
	"testing"

	"example.com/testvariantaspect/helper"
	"example.com/testvariantaspect/subject"
)

// instrumented is set by the instrumentation aspect when value is called.
var instrumented bool

func value() int { return subject.Value() }

func TestExternal(t *testing.T) {
	if got := subject.Value(); got != 42 {
		t.Fatalf("subject.Value() = %d, want 42", got)
	}
	if !subject.Instrumented {
		t.Error("aspects were not applied to the test variant imported by the external test package")
	}

	if got := value(); got != 42 {
		t.Fatalf("value() = %d, want 42", got)
	}
	if !instrumented {
		t.Error("aspects were not applied to the external test package")
	}

	if got := helper.Doubled(); got != 84 {
		t.Fatalf("helper.Doubled() = %d, want 84", got)
	}
	if !helper.Instrumented {
		t.Error("aspects were not applied to the importer Go rebuilt for this test binary")
	}
}
`)

	orchestrion := buildOrchestrion(t)
	run.exec(t, orchestrion, "go", "test", "-a", "./subject")
	// Coverage-enabled builds hand the compiler sources rewritten by `go tool cover` instead of the
	// package's own, which must not prevent aspects from applying either.
	run.exec(t, orchestrion, "go", "test", "-a", "-coverprofile="+filepath.Join(run.dir, "coverage.out"), "./subject")
}

func TestBuildFromModuleSubdirectory(t *testing.T) {
	run := runner{dir: t.TempDir()}

	run.exec(t, "go", "mod", "init", "github.com/DataDog/orchestrion.testing")
	run.exec(t, "go", "mod", "edit", "-replace=github.com/DataDog/orchestrion="+rootDir)
	require.NoError(t, os.Mkdir(filepath.Join(run.dir, "cmd"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(run.dir, "cmd", "main.go"), []byte(`package main

import (
	"log"

	"github.com/DataDog/orchestrion/runtime/built"
)

func main() {
	if !built.WithOrchestrion {
		log.Fatalln("Not built with orchestrion 🤨")
	}
}
`), 0o644))
	orchestrionBin := buildOrchestrion(t)
	run.exec(t, orchestrionBin, "pin")

	// Run the command from a working directory that is NOT the module root, so we can ensure the
	// configuration is appropriately loaded from the module's root anyway.
	runCmd := runner{dir: filepath.Join(run.dir, "cmd")}
	runCmd.exec(t, orchestrionBin, "go", "run", ".")
}

type benchCase interface {
	baseline(b *testing.B)
	instrumented(b *testing.B)
}

var benchCases = map[string]func(b *testing.B) benchCase{
	"DataDog:orchestrion": benchmarkOrchestrion,
	// normal build
	"traefik:traefik": benchmarkGithub("traefik", "traefik", "", "./...", false),
	"go-delve:delve":  benchmarkGithub("go-delve", "delve", "", "./...", false),
	"jlegrone:tctx":   benchmarkGithub("jlegrone", "tctx", "", "./...", false),
	"tinylib:msgp":    benchmarkGithub("tinylib", "msgp", "", "./...", false),
	"etcd-io:etcd":    benchmarkGithub("etcd-io", "etcd", "server", "./...", false),
	// test packages
	"gin-gonic:gin.test": benchmarkGithub("gin-gonic", "gin", "", "./...", true),
	"jlegrone:tctx.test": benchmarkGithub("jlegrone", "tctx", "", "./...", true),
}

func Benchmark(b *testing.B) {
	for name, create := range benchCases {
		b.Run("repo="+name, func(b *testing.B) {
			tc := create(b)
			b.Run("variant=baseline", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					tc.baseline(b)
				}
			})

			b.Run("variant=instrumented", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					tc.instrumented(b)
				}
			})
		})
	}
}

type benchGithub struct {
	harness
}

// benchmarkGithub builds a benchmark case for a github repo. If subdir is
// non-empty, setup and build run against that sub-module (etcd-style monorepos)
// rather than the repo root.
func benchmarkGithub(owner string, repo string, subdir string, build string, testbuild bool) func(b *testing.B) benchCase {
	return func(b *testing.B) benchCase {
		tc := &benchGithub{harness{build: build, testbuild: testbuild}}

		tag := tc.findLatestGithubReleaseTag(b, owner, repo)
		b.Logf("Latest release is %s/%s@%s", owner, repo, tag)

		tc.gitCloneGithub(b, owner, repo, tag)
		if subdir != "" {
			tc.dir = filepath.Join(tc.dir, subdir)
		}
		tc.exec(b, "go", "mod", "download")
		tc.exec(b, "go", "mod", "edit", "-replace=github.com/DataDog/orchestrion="+rootDir)
		if replace := os.Getenv("DD_TRACE_GO_REPLACE"); replace != "" {
			applyDDTraceGoReplaces(b, &tc.runner, replace)
		}
		if stat, err := os.Stat(filepath.Join(tc.dir, "vendor")); err == nil && stat.IsDir() {
			// If there's a vendor dir, we need to update the `modules.txt` in there to reflect the replacement.
			tc.exec(b, "go", "mod", "vendor")
		}
		// traefik needs a few tweaks in order to build successfully
		if repo == "traefik" {
			// it fails to build if ./webui/static does not exist, so just create a folder with mock content
			webuiPath := filepath.Join(tc.dir, "webui")
			if stat, err := os.Stat(webuiPath); err == nil && stat.IsDir() {
				staticPath := filepath.Join(webuiPath, "static")
				err := os.MkdirAll(staticPath, 0755)
				require.NoError(b, err, "failed to create static directory for traefik build: %s", staticPath)

				indexFile := filepath.Join(staticPath, "index.html")
				f, err := os.Create(indexFile)
				require.NoError(b, err, "failed to create mock content for traefik build: %s", indexFile)
				require.NoError(b, f.Close())
			}
		}
		tc.exec(b, buildOrchestrion(b), "pin")

		return tc
	}
}

type benchOrchestrion struct {
	harness
}

func benchmarkOrchestrion(_ *testing.B) benchCase {
	return &benchOrchestrion{harness{runner: runner{dir: rootDir}, build: ".", testbuild: false}}
}

type runner struct {
	dir string // The directory where commands are to be executed
}

type harness struct {
	runner
	build     string // The package to be built as part of the test.
	testbuild bool   // Whether the package to be built is a test package.
}

func (h *harness) baseline(b *testing.B) {
	b.Helper()

	var cmd *exec.Cmd
	if h.testbuild {
		cmd = exec.Command("go", "test", "-c", "-o", b.TempDir(), h.build)
	} else {
		cmd = exec.Command("go", "build", "-o", b.TempDir(), h.build)
	}
	cmd.Dir = h.dir
	cmd.Env = append(os.Environ(), "GOCACHE="+b.TempDir())
	output := bytes.NewBuffer(make([]byte, 0, 4_096))
	cmd.Stdout = output
	cmd.Stderr = output

	b.StartTimer()
	err := cmd.Run()
	b.StopTimer()

	require.NoError(b, err, "build failed:\n%s\n%s", cmd, output)
}

func (h *harness) instrumented(b *testing.B) {
	b.Helper()

	var cmd *exec.Cmd
	if h.testbuild {
		cmd = exec.Command(buildOrchestrion(b), "go", "test", "-c", "-o", b.TempDir(), h.build)
	} else {
		cmd = exec.Command(buildOrchestrion(b), "go", "build", "-o", b.TempDir(), h.build)
	}
	cmd.Dir = h.dir
	cmd.Env = append(os.Environ(), "GOCACHE="+b.TempDir())
	output := bytes.NewBuffer(make([]byte, 0, 4_096))
	cmd.Stdout = output
	cmd.Stderr = output

	b.StartTimer()
	err := cmd.Run()
	b.StopTimer()

	require.NoError(b, err, "build failed:\n%s", output)
}

func (r *runner) exec(tb testing.TB, name string, args ...string) {
	r.execWithCache(tb, tb.TempDir(), name, args...)
}

func (r *runner) execWithCache(tb testing.TB, cache string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GOCACHE="+cache)
	output := bytes.NewBuffer(make([]byte, 0, 4_096))
	cmd.Stdout = output
	cmd.Stderr = output

	require.NoError(tb, cmd.Run(), "command failed: %s\n%s", cmd, output)
}

func (r *runner) execError(tb testing.TB, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GOCACHE="+tb.TempDir())
	output := bytes.NewBuffer(make([]byte, 0, 4_096))
	cmd.Stdout = output
	cmd.Stderr = output

	require.Error(tb, cmd.Run(), "command succeeded unexpectedly: %s\n%s", cmd, output)
	return output.String()
}

func (*harness) findLatestGithubReleaseTag(b *testing.B, owner string, repo string) string {
	// NB -- Default page size is 30, and releases are sorted by creation date... We should be able to rely on the tag
	// we are looking for being present in the first page, ergo we don't bother traversing all pages.
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo), nil)
	require.NoError(b, err)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if token, ok := getGithubToken(); ok {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(b, err)
	defer resp.Body.Close()

	require.Equal(b, http.StatusOK, resp.StatusCode, "error response body:\n%s", contentString{resp.Body})

	var payload struct {
		TagName string `json:"tag_name"`
	}

	require.NoError(b, json.NewDecoder(resp.Body).Decode(&payload))
	require.NotEmpty(b, payload)

	return payload.TagName
}

func requireCoverageCountAtLeast(t *testing.T, profilePath string, fileName string, mode string, minimum int) {
	t.Helper()

	profiles, err := cover.ParseProfiles(profilePath)
	require.NoError(t, err)
	for _, profile := range profiles {
		if filepath.ToSlash(profile.FileName) != fileName {
			continue
		}
		require.Equal(t, mode, profile.Mode)
		for _, block := range profile.Blocks {
			if block.Count >= minimum {
				return
			}
		}
		require.Failf(t, "coverage count is too low", "%s has no block with a count of at least %d: %#v", fileName, minimum, profile.Blocks)
	}
	require.Failf(t, "coverage profile is missing a file", "%s does not contain %s", profilePath, fileName)
}

func getGithubToken() (string, bool) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, true
	}

	var bytes bytes.Buffer
	cmd := exec.Command("gh", "auth", "token")
	cmd.Stdout = &bytes
	cmd.Stderr = &bytes

	if err := cmd.Run(); err != nil {
		return "", false
	}

	return strings.TrimSpace(bytes.String()), true
}

// applyDDTraceGoReplaces walks replace, finds every go.mod whose module path
// starts with github.com/DataDog/dd-trace-go, and adds a corresponding replace
// directive to the target's go.mod. This lets the benchmarks build against a
// local checkout of dd-trace-go (including branches where transitive contribs
// reference an unpublished placeholder version like v2.10.0-dev).
func applyDDTraceGoReplaces(b *testing.B, r *runner, replace string) {
	b.Helper()

	err := filepath.WalkDir(replace, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != replace {
				name := d.Name()
				if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mod, err := modfile.Parse(path, data, nil)
		if err != nil {
			return err
		}
		if mod.Module == nil {
			return nil
		}
		modPath := mod.Module.Mod.Path
		if !strings.HasPrefix(modPath, "github.com/DataDog/dd-trace-go") {
			return nil
		}
		r.exec(b, "go", "mod", "edit", "-replace="+modPath+"="+filepath.Dir(path))
		return nil
	})
	require.NoError(b, err)
}

func (h *harness) gitCloneGithub(b *testing.B, owner string, repo string, tag string) string {
	b.Helper()

	h.dir = b.TempDir()
	h.exec(b, "git", "clone", "--depth=1", "--branch="+tag, fmt.Sprintf("https://github.com/%s/%s.git", owner, repo), h.dir)

	return h.dir
}

var (
	orchestrionBinOnce sync.Once
	orchestrionBin     string
)

func buildOrchestrion(tb testing.TB) string {
	tb.Helper()

	orchestrionBinOnce.Do(func() {
		orchestrionBin = filepath.Join(rootDir, "bin", "orchestrion.exe")

		cmd := exec.Command("go", "build", "-o="+orchestrionBin, rootDir)
		require.NoError(tb, cmd.Run())
	})

	return orchestrionBin
}

type contentString struct{ io.Reader }

func (c contentString) String() string {
	data, _ := io.ReadAll(c)
	return string(data)
}

var rootDir string

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	rootDir = filepath.Dir(thisFile)
}
