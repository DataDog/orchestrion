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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

type testCase interface {
	baseline()
	instrumented()
}

var testCases = map[string]func(b *testing.B) testCase{
	"DataDog:orchestrion": benchmarkOrchestrion,
	"traefik:traefik":     benchmarkGithub("traefik", "traefik", "./..."),
	"go-delve:delve":      benchmarkGithub("go-delve", "delve", "./..."),
	"jlegrone:tctx":       benchmarkGithub("jlegrone", "tctx", "./..."),
	"tinylib:msgp":        benchmarkGithub("tinylib", "msgp", "./..."),
}

func Benchmark(b *testing.B) {
	for name, create := range testCases {
		tc := create(b)
		b.Run(fmt.Sprintf("repo=%s/variant=baseline", name), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.baseline()
			}
		})

		b.Run(fmt.Sprintf("repo=%s/variant=instrumented", name), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tc.instrumented()
			}
		})
	}
}

type benchGithub struct {
	harness
}

func benchmarkGithub(owner, repo, build string) func(b *testing.B) testCase {
	return func(b *testing.B) testCase {
		b.Helper()

		tc := &benchGithub{harness{B: b, build: build}}

		tag := tc.findLatestGithubReleaseTag(owner, repo)
		tc.gitCloneGithub(owner, repo, tag)
		tc.exec("go", "mod", "download")
		tc.exec("go", "mod", "edit", "-replace", fmt.Sprintf("github.com/DataDog/orchestrion=%s", rootDir))
		tc.exec(buildOrchestrion(b), "pin")
		tc.exec("go", "mod", "vendor")

		return tc
	}
}

type benchOrchestrion struct {
	harness
}

func benchmarkOrchestrion(b *testing.B) testCase {
	return &benchOrchestrion{harness{B: b, dir: rootDir, build: "."}}
}

type harness struct {
	*testing.B
	dir   string // The directory in which the source code of the package to be built is located.
	build string // The package to be built as part of the test.
}

func (h *harness) baseline() {
	h.Helper()

	cmd := exec.Command("go", "build", "-o", h.TempDir(), h.build)
	cmd.Dir = h.dir
	cmd.Env = append(os.Environ(), "GOCACHE="+h.TempDir())
	output := bytes.NewBuffer(make([]byte, 0, 4_096))
	cmd.Stdout = output
	cmd.Stderr = output

	h.StartTimer()
	err := cmd.Run()
	h.StopTimer()

	assert.NoError(h, err, "build failed:\n%s", output)
}

func (h *harness) instrumented() {
	h.Helper()

	cmd := exec.Command(buildOrchestrion(h.B), "go", "build", "-o", h.TempDir(), h.build)
	cmd.Dir = h.dir
	cmd.Env = append(os.Environ(), "GOCACHE="+h.TempDir())
	output := bytes.NewBuffer(make([]byte, 0, 4_096))
	cmd.Stdout = output
	cmd.Stderr = output

	h.StartTimer()
	err := cmd.Run()
	h.StopTimer()

	assert.NoError(h, err, "build failed:\n%s", output)
}

func (h *harness) exec(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = h.dir
	cmd.Env = append(os.Environ(), "GOCACHE="+h.TempDir())
	output := bytes.NewBuffer(make([]byte, 0, 4_096))
	cmd.Stdout = output
	cmd.Stderr = output

	require.NoError(h, cmd.Run(), "command failed: %s\n%s", cmd, output)
}

func (h *harness) findLatestGithubReleaseTag(owner string, repo string) string {
	h.Helper()

	// NB -- Default page size is 30, and releases are sorted by creation date... We should be able to rely on the tag
	// we are looking for being present in the first page, ergo we don't bother traversing all pages.
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo), nil)
	require.NoError(h, err)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if token, ok := getGithubToken(); ok {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(h, err)
	defer resp.Body.Close()

	require.Equal(h, http.StatusOK, resp.StatusCode, "error response body:\n%s", contentString{resp.Body})

	var payload []struct {
		Prerelease bool   `json:"prerelease"`
		TagName    string `json:"tag_name"`
	}
	require.NoError(h, json.NewDecoder(resp.Body).Decode(&payload))
	require.NotEmpty(h, payload)

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

	require.NotEmpty(h, tagName)

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

	return bytes.String(), true
}

func (h *harness) gitCloneGithub(owner string, repo string, tag string) string {
	h.Helper()

	h.dir = h.TempDir()
	h.exec("git", "clone", "--depth=1", fmt.Sprintf("--branch=%s", tag), fmt.Sprintf("https://github.com/%s/%s.git", owner, repo), h.dir)

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
