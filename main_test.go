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
	"golang.org/x/mod/semver"
)

type testCase interface {
	baseline(b *testing.B)
	instrumented(b *testing.B)
}

var testCases = map[string]func(b *testing.B) testCase{
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
	for name, create := range testCases {
		b.Run(fmt.Sprintf("repo=%s", name), func(b *testing.B) {
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

func benchmarkGithub(owner string, repo string, build string, testbuild bool) func(b *testing.B) testCase {
	return func(b *testing.B) testCase {
		b.Helper()

		tc := &benchGithub{harness{build: build, testbuild: testbuild}}

		tag := tc.findLatestGithubReleaseTag(b, owner, repo)
		tc.gitCloneGithub(b, owner, repo, tag)
		tc.exec(b, "go", "mod", "download")
		tc.exec(b, "go", "mod", "edit", "-replace", fmt.Sprintf("github.com/DataDog/orchestrion=%s", rootDir))
		if stat, err := os.Stat(filepath.Join(tc.dir, "vendor")); err == nil && stat.IsDir() {
			// If there's a vendor dir, we need to update the `modules.txt` in there to reflect the replacement.
			tc.exec(b, "go", "mod", "vendor")
		}
		tc.exec(b, buildOrchestrion(b), "pin")

		return tc
	}
}

type benchOrchestrion struct {
	harness
}

func benchmarkOrchestrion(_ *testing.B) testCase {
	return &benchOrchestrion{harness{dir: rootDir, build: ".", testbuild: false}}
}

type harness struct {
	dir       string // The directory in which the source code of the package to be built is located.
	build     string // The package to be built as part of the test.
	testbuild bool   // Whether the package to be built is a test package.
}

func (h *harness) baseline(b *testing.B) {
	b.Helper()

	var cmd *exec.Cmd
	if h.testbuild {
		cmd = exec.Command("go", "test", "-c", "-o", h.TempDir(), h.build)
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

	require.NoError(b, err, "build failed:\n%s", output)
}

func (h *harness) instrumented(b *testing.B) {
	b.Helper()

	var cmd *exec.Cmd
	if h.testbuild {
		cmd = exec.Command(buildOrchestrion(h.B), "go", "test", "-c", "-o", h.TempDir(), h.build)
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

func (h *harness) exec(b *testing.B, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = h.dir
	cmd.Env = append(os.Environ(), "GOCACHE="+b.TempDir())
	output := bytes.NewBuffer(make([]byte, 0, 4_096))
	cmd.Stdout = output
	cmd.Stderr = output

	require.NoError(b, cmd.Run(), "command failed: %s\n%s", cmd, output)
}

func (*harness) findLatestGithubReleaseTag(b *testing.B, owner string, repo string) string {
	b.Helper()

	// NB -- Default page size is 30, and releases are sorted by creation date... We should be able to rely on the tag
	// we are looking for being present in the first page, ergo we don't bother traversing all pages.
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo), nil)
	require.NoError(b, err)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if token, ok := getGithubToken(); ok {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(b, err)
	defer resp.Body.Close()

	require.Equal(b, http.StatusOK, resp.StatusCode, "error response body:\n%s", contentString{resp.Body})

	var payload []struct {
		Prerelease bool   `json:"prerelease"`
		TagName    string `json:"tag_name"`
	}
	require.NoError(b, json.NewDecoder(resp.Body).Decode(&payload))
	require.NotEmpty(b, payload)

	var tagName string
	for _, release := range payload {
		if release.Prerelease {
			// We're excluding pre-releases, just because.
			continue
		}
		if tagName == "" || semver.Compare(tagName, release.TagName) < 0 {
			tagName = release.TagName
		}
	}

	require.NotEmpty(b, tagName)

	return tagName
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
	h.exec(b, "git", "clone", "--depth=1", fmt.Sprintf("--branch=%s", tag), fmt.Sprintf("https://github.com/%s/%s.git", owner, repo), h.dir)

	return h.dir
}

var (
	orchestrionBinOnce sync.Once
	orchestrionBin     string
)

func buildOrchestrion(b *testing.B) string {
	b.Helper()

	orchestrionBinOnce.Do(func() {
		orchestrionBin = filepath.Join(rootDir, "bin", "orchestrion.exe")

		cmd := exec.Command("go", "build", fmt.Sprintf("-o=%s", orchestrionBin), rootDir)
		require.NoError(b, cmd.Run())
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
