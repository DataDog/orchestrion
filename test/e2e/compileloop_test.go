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
	"testing"
	"time"

	"github.com/DataDog/orchestrion/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildBudget is how long the reproduction build is given to complete. The
// fixture is tiny and depends on no tracer integration, but `-toolexec` builds
// re-compile the standard library, so a cold build cache still needs minutes.
const buildBudget = 4 * time.Minute

// TestCompileLoop builds a package that is instrumented with a dependency on a
// package that depends on it, which is what happens when a package's own tests
// exercise a call site one of its integrations instruments.
//
// Resolving that injected dependency spawns a nested build, which has to compile
// the instrumented package again, with the very same build ID. The job server's
// never-build-twice service then makes it wait for the in-flight compilation
// that spawned it, and that one cannot complete until the nested build does:
// nothing ever unblocks, and no timeout exists.
func TestCompileLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e compile loop test in short mode")
	}

	orchestrionBin := e2e.FindOrchestrionBinary(t)
	t.Logf("Using orchestrion binary: %s", orchestrionBin)

	workDir := e2e.CreateWorkDir(t, "testdata/compileloop")

	logFile, err := os.Create(filepath.Join(workDir, "test.log"))
	require.NoError(t, err)
	defer logFile.Close()
	log := e2e.Logger(t, logFile)

	binary := filepath.Join(workDir, "compileloop")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	ctx, cancel := context.WithTimeout(context.Background(), buildBudget)
	defer cancel()

	cmd := exec.CommandContext(ctx, orchestrionBin, "go", "build", "-o", binary, ".")
	cmd.Dir = workDir
	// The fixture provides its own `orchestrion.tool.go`, and needs no tracer
	// integration at all: skip automatic pinning so that the reproduction neither
	// reaches out to the network nor builds anything it does not need.
	cmd.Env = append(os.Environ(), "DD_ORCHESTRION_IS_GOMOD_VERSION=true")
	// The deadlocked toolexec processes keep the output pipes open, so stop waiting
	// for them shortly after the build itself was killed.
	cmd.WaitDelay = 30 * time.Second

	log("Building %q with a budget of %v...", workDir, buildBudget)
	start := time.Now()
	output, err := cmd.CombinedOutput()
	log("Build returned after %v", time.Since(start))

	buildLog := filepath.Join(workDir, "build.log")
	require.NoError(t, os.WriteFile(buildLog, output, 0o644))

	require.NoError(t, ctx.Err(),
		"`orchestrion go build` made no progress within %v: it is waiting for the compilation of a package that only this build can complete.\nBuild output:\n%s",
		buildBudget,
		output,
	)

	// Failing is a valid outcome, as the injected dependency cycle cannot be
	// satisfied; but it must be reported instead of left for the user to guess.
	if err != nil {
		log("Build failed: %v", err)
		assert.Contains(t, string(output), "cycle detected",
			"the build failure must explain the dependency cycle instead of being cryptic")
		return
	}

	log("Build succeeded, the injected dependency cycle was satisfied")
	require.FileExists(t, binary)
}
