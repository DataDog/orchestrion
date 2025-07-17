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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	// configuration is appopriately loaded from the module's root anyway.
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
	"traefik:traefik": benchmarkGithub("traefik", "traefik", "./...", false),
	"go-delve:delve":  benchmarkGithub("go-delve", "delve", "./...", false),
	"jlegrone:tctx":   benchmarkGithub("jlegrone", "tctx", "./...", false),
	"tinylib:msgp":    benchmarkGithub("tinylib", "msgp", "./...", false),
	// test packages
	"gin-gonic:gin.test": benchmarkGithub("gin-gonic", "gin", "./...", true),
	"jlegrone:tctx.test": benchmarkGithub("jlegrone", "tctx", "./...", true),
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

func benchmarkGithub(owner string, repo string, build string, testbuild bool) func(b *testing.B) benchCase {
	return func(b *testing.B) benchCase {
		b.Helper()

		tc := &benchGithub{harness{build: build, testbuild: testbuild}}

		tag := tc.findLatestGithubReleaseTag(b, owner, repo)
		b.Logf("Latest release is %s/%s@%s", owner, repo, tag)

		tc.gitCloneGithub(b, owner, repo, tag)
		tc.exec(b, "go", "mod", "download")
		tc.exec(b, "go", "mod", "edit", "-replace=github.com/DataDog/orchestrion="+rootDir)
		if stat, err := os.Stat(filepath.Join(tc.dir, "vendor")); err == nil && stat.IsDir() {
			// If there's a vendor dir, we need to update the `modules.txt` in there to reflect the replacement.
			tc.exec(b, "go", "mod", "vendor")
		}
		// traefik fails to build if we don't upgrade the version go.opentelemetry.io/otel/sdk/log
		if repo == "traefik" {
			tc.exec(b, "go", "get", "go.opentelemetry.io/otel/sdk/log@latest")
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
	cmd := exec.Command(name, args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GOCACHE="+tb.TempDir())
	output := bytes.NewBuffer(make([]byte, 0, 4_096))
	cmd.Stdout = output
	cmd.Stderr = output

	require.NoError(tb, cmd.Run(), "command failed: %s\n%s", cmd, output)
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
