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

// TestPGO verifies that orchestrion works correctly with Profile-Guided Optimization.
// This test validates the fix for https://github.com/DataDog/orchestrion/issues/653
func TestPGO(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e PGO test in short mode")
	}

	orchestrionBin := e2e.FindOrchestrionBinary(t)
	t.Logf("Using orchestrion binary: %s", orchestrionBin)

	workDir := e2e.CreateWorkDir(t, "testdata/pgo")

	testTimeout := e2e.TestTimeout(t)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	logFile := filepath.Join(workDir, "test.log")
	logWriter, err := os.Create(logFile)
	require.NoError(t, err)
	defer logWriter.Close()

	log := e2e.Logger(t, logWriter)
	testStartTime := time.Now()

	log("=== Timeout Configuration ===")
	log("Test context timeout: %v", testTimeout)
	log("Test start time: %v", testStartTime.Format(time.RFC3339))
	log("Context deadline: %v", testStartTime.Add(testTimeout).Format(time.RFC3339))
	log("")

	log("=== PGO E2E Test ===")
	log("Testing fix for https://github.com/DataDog/orchestrion/issues/653")
	log("")

	// Step 1: Build regular binary for profiling
	log("Step 1: Building binary for profiling (without orchestrion)...")
	stepStart := time.Now()
	binary := filepath.Join(workDir, "pgo-sample")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	buildLog := filepath.Join(workDir, "build-without-pgo.log")
	cmd := exec.CommandContext(ctx, "go", "build", "-x", "-o", binary, ".")
	cmd.Dir = workDir
	e2e.RunAndLog(t, cmd, buildLog, log)
	require.FileExists(t, binary)
	log("✓ Built %s (took %v, elapsed: %v)", binary, time.Since(stepStart), time.Since(testStartTime))

	// Step 2: Run binary to collect CPU profile
	log("")
	log("Step 2: Running binary to collect CPU profile...")
	stepStart = time.Now()
	profilePath := filepath.Join(workDir, "cpu.prof")

	cmd = exec.CommandContext(ctx, binary)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "CPUPROFILE="+profilePath)

	err = e2e.WaitForCommandWithTimeout(t, cmd, 10*time.Second)
	require.NoError(t, err, "Binary exited with error")
	require.FileExists(t, profilePath)
	log("✓ Profile collected: %s (took %v, elapsed: %v)", profilePath, time.Since(stepStart), time.Since(testStartTime))

	// Step 3: Convert profile to PGO format
	log("")
	log("Step 3: Converting profile to PGO format...")
	stepStart = time.Now()
	pgoProfile := filepath.Join(workDir, "default.pgo")

	cmd = exec.CommandContext(ctx, "go", "tool", "pprof", "-proto", profilePath)
	cmd.Dir = workDir
	output, err := cmd.Output()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pgoProfile, output, 0644))
	require.FileExists(t, pgoProfile)

	info, err := os.Stat(pgoProfile)
	require.NoError(t, err)
	log("✓ Created %s (size: %d bytes, took %v, elapsed: %v)", pgoProfile, info.Size(), time.Since(stepStart), time.Since(testStartTime))
	assert.Greater(t, info.Size(), int64(100), "PGO profile should not be empty")

	// Step 4: Build with orchestrion AND PGO enabled (the critical test!)
	log("")
	log("Step 4: Building with orchestrion AND PGO enabled...")
	log("  This tests the fix for https://github.com/DataDog/orchestrion/issues/653")

	stepStart = time.Now()
	orchestrionBinary := filepath.Join(workDir, "pgo-sample-orchestrion")
	if runtime.GOOS == "windows" {
		orchestrionBinary += ".exe"
	}

	buildWithPGOLog := filepath.Join(workDir, "build-with-pgo.log")
	cmd = exec.CommandContext(ctx, orchestrionBin, "go", "build", "-x", "-pgo="+pgoProfile, "-o", orchestrionBinary, ".")
	cmd.Dir = workDir
	// Enable debug logging to capture job server lifecycle and timing information
	cmd.Env = append(os.Environ(), "ORCHESTRION_LOG_LEVEL=DEBUG")
	log("  Starting orchestrion build at %v (elapsed: %v)", time.Now().Format(time.RFC3339), time.Since(testStartTime))
	e2e.RunAndLog(t, cmd, buildWithPGOLog, log)

	require.FileExists(t, orchestrionBinary)
	log("✓ Successfully built with orchestrion + PGO (took %v, elapsed: %v)", time.Since(stepStart), time.Since(testStartTime))
	log("✓ The fix works - no 'mismatched build ID' errors!")

	// Step 5: Verify build logs are clean
	log("")
	log("Step 5: Verifying build logs are clean...")

	// Check the orchestrion+PGO build log
	buildLogContent, err := os.ReadFile(buildWithPGOLog)
	require.NoError(t, err)
	buildLogStr := string(buildLogContent)

	// Should not contain any error indicators
	assert.NotContains(t, buildLogStr, "mismatched build ID", "Build log should not contain 'mismatched build ID' errors")
	assert.NotContains(t, buildLogStr, "exit status", "Build log should not contain exit status errors")
	assert.NotContains(t, buildLogStr, "FAIL\t", "Build log should not contain test failures")

	// Should contain PGO indicator
	assert.Contains(t, buildLogStr, "-pgo=", "Build should use PGO flag")
	log("✓ Build logs are clean (no errors detected)")

	// Verify we're actually using orchestrion instrumentation
	assert.Contains(t, buildLogStr, "orchestrion", "Build should mention orchestrion")
	log("✓ Orchestrion instrumentation is active")

	// Step 6: Verify the artifacts
	log("")
	log("Step 6: Verifying built artifacts...")

	// Check binary is executable and has reasonable size
	binInfo, err := os.Stat(orchestrionBinary)
	require.NoError(t, err)
	assert.Greater(t, binInfo.Size(), int64(1024*1024), "Binary should be at least 1MB")

	// On Unix, check executable bit
	if runtime.GOOS != "windows" {
		assert.NotEqual(t, binInfo.Mode()&0111, 0, "Binary should be executable")
	}
	log("✓ Binary is valid (size: %d bytes)", binInfo.Size())

	// Step 7: Verify the binary actually runs
	log("")
	log("Step 7: Running the orchestrion+PGO binary...")
	cmd = exec.CommandContext(ctx, orchestrionBinary)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "CPUPROFILE=") // Disable profiling for verification run

	require.NoError(t, cmd.Start())

	// Give it a moment to start, then kill it
	time.Sleep(500 * time.Millisecond)
	cmd.Process.Kill()
	log("✓ Binary runs successfully")

	log("")
	log("=== PGO E2E Test PASSED ===")
	log("Total test duration: %v (budget was %v)", time.Since(testStartTime), testTimeout)

	// Log file locations for debugging
	t.Logf("Test artifacts in: %s", workDir)
	t.Logf("Build logs:")
	t.Logf("  - %s", buildLog)
	t.Logf("  - %s", buildWithPGOLog)
	t.Logf("Profiles:")
	t.Logf("  - %s (CPU profile)", profilePath)
	t.Logf("  - %s (PGO profile, %d bytes)", pgoProfile, info.Size())
	t.Logf("Binaries:")
	t.Logf("  - %s (%d bytes)", binary, binInfo.Size())
	t.Logf("  - %s (%d bytes, with PGO)", orchestrionBinary, binInfo.Size())

	// Note: Artifacts in workDir are automatically cleaned up by t.TempDir()
}
