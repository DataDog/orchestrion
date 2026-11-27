// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/orchestrion/test/e2e"
	"github.com/stretchr/testify/require"
)

// buildBudget is how long a single orchestrion-driven test compilation is given
// to complete. The fixture is tiny, but `-toolexec` builds re-compile the
// standard library, so a cold build cache still needs minutes.
const lineInfoBuildBudget = 10 * time.Minute

// TestLineInfo builds a fixture that mixes cgo, one-line function bodies, and
// function literals, and asserts that the runtime line numbers of woven calls
// match the plain Go source lines: the reported cases of DataDog/orchestrion#948.
//
// The cgo part needs a working C toolchain: the test reports an explicit skip
// where cgo is disabled or no C compiler is available.
func TestLineInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e line info test in short mode")
	}
	if !cgoAvailable(t) {
		t.Skip("Skipping e2e line info test: cgo is disabled or no C compiler is available")
	}

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	// Build the orchestrion binary fresh so the test always exercises the
	// current working tree, never a stale bin/orchestrion.
	orchestrionBin := filepath.Join(repoRoot, "bin", "orchestrion")
	if runtime.GOOS == "windows" {
		orchestrionBin += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", orchestrionBin, ".")
	build.Dir = repoRoot
	out, err := build.CombinedOutput()
	require.NoError(t, err, "Failed to build orchestrion:\n%s", out)

	workDir := e2e.CreateWorkDir(t, "testdata/lineinfo")
	logFile, err := os.Create(filepath.Join(workDir, "test.log"))
	require.NoError(t, err)
	defer logFile.Close()
	log := e2e.Logger(t, logFile)

	log("Testing line information in %q with a fresh orchestrion binary", workDir)

	testCtx, testCancel := context.WithTimeout(context.Background(), lineInfoBuildBudget)
	defer testCancel()
	cmd := exec.CommandContext(testCtx, orchestrionBin, "go", "test", "-count=1", "./...")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		// The fixture provides its own `orchestrion.tool.go`, and needs no tracer
		// integration at all: skip automatic pinning so the reproduction neither
		// reaches out to the network nor builds anything it does not need.
		"DD_ORCHESTRION_IS_GOMOD_VERSION=true",
	)
	cmd.WaitDelay = 30 * time.Second

	start := time.Now()
	output, err := cmd.CombinedOutput()
	log("orchestrion go test returned after %v", time.Since(start))

	require.NoError(t, testCtx.Err(),
		"`orchestrion go test` made no progress within %v.\nOutput:\n%s", lineInfoBuildBudget, output)

	testLog := filepath.Join(workDir, "test-output.log")
	require.NoError(t, os.WriteFile(testLog, output, 0o644))

	require.NoError(t, err,
		"woven runtime line numbers do not match the plain Go source lines (DataDog/orchestrion#948).\nLog: %s\nOutput:\n%s",
		testLog, output)
}

// cgoAvailable reports whether cgo builds can run on this host: cgo must be
// enabled and the C compiler must be present.
func cgoAvailable(t *testing.T) bool {
	t.Helper()

	out, err := exec.Command("go", "env", "CGO_ENABLED", "CC").Output()
	if err != nil {
		t.Logf("failed to inspect the go environment: %v", err)
		return false
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 || fields[0] != "1" {
		return false
	}
	if fields[1] == "" {
		return false
	}
	if _, err := exec.LookPath(fields[1]); err != nil {
		return false
	}
	return true
}
