// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Ensure fmt is used
var _ = fmt.Sprint

// FindOrchestrionBinary locates or builds the orchestrion binary for testing
func FindOrchestrionBinary(t *testing.T) string {
	t.Helper()

	// Find the repository root (where go.mod for orchestrion is)
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "Failed to find repository root")

	binPath := filepath.Join(repoRoot, "bin", "orchestrion")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}

	// Try to build it
	t.Log("Orchestrion binary not found, building...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build orchestrion:\n%s", output)

	return binPath
}

// findRepoRoot finds the orchestrion repository root by looking for go.mod
func findRepoRoot() (string, error) {
	// Start from current directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up until we find the orchestrion go.mod (not a test one)
	for {
		gomodPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(gomodPath); err == nil {
			// Check if this is the main orchestrion module
			if strings.Contains(string(data), "module github.com/DataDog/orchestrion\n") {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find orchestrion repository root")
		}
		dir = parent
	}
}

// RunAndLog executes a command, logs output to a file, and checks for errors
func RunAndLog(t *testing.T, cmd *exec.Cmd, logPath string, log func(string, ...interface{})) {
	t.Helper()

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	// Write to log file
	if logErr := os.WriteFile(logPath, output, 0644); logErr != nil {
		t.Logf("Warning: Failed to write log file %s: %v", logPath, logErr)
	}

	// Check for "mismatched build ID" error (the bug we fixed)
	if len(output) > 0 {
		outputStr := string(output)
		if strings.Contains(outputStr, "mismatched build ID") {
			t.Errorf("Found 'mismatched build ID' error in build output - the fix didn't work!")
			t.Logf("Build output:\n%s", outputStr)
			t.FailNow()
		}
	}

	require.NoError(t, err, "Command failed: %s\nLog file: %s\nOutput:\n%s",
		cmd.String(), logPath, string(output))

	if log != nil {
		log("  Command succeeded (log: %s)", filepath.Base(logPath))
	}
}

// CopyDir recursively copies a directory from src to dst
func CopyDir(t *testing.T, src, dst string) {
	t.Helper()

	entries, err := os.ReadDir(src)
	require.NoError(t, err)

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Skip hidden directories and common non-source directories
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			require.NoError(t, os.MkdirAll(dstPath, 0755))
			CopyDir(t, srcPath, dstPath)
		} else {
			// Copy file
			data, err := os.ReadFile(srcPath)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(dstPath, data, 0644))
		}
	}
}

// Logger creates a logging function that writes to both test output and a log file
func Logger(t *testing.T, logFile *os.File) func(string, ...interface{}) {
	t.Helper()
	return func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		t.Log(msg)
		if logFile != nil {
			fmt.Fprintln(logFile, msg)
		}
	}
}

// CreateWorkDir creates a timestamped work directory in the repo and copies test files into it
func CreateWorkDir(t *testing.T, testDir string) string {
	t.Helper()

	// Find repo root
	repoRoot, err := findRepoRoot()
	require.NoError(t, err)

	// Create work directory in repo's test/e2e/work/
	workBase := filepath.Join(repoRoot, "test", "e2e", "work")
	require.NoError(t, os.MkdirAll(workBase, 0755))

	// Create timestamped directory for this test run
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	workDir := filepath.Join(workBase, fmt.Sprintf("%s-%s", t.Name(), timestamp))
	require.NoError(t, os.MkdirAll(workDir, 0755))

	// Clean up old directories for this test, keeping only recent 5
	cleanupOldWorkDirs(t, workBase, t.Name(), 5)

	// Clean up at test end if test passed
	t.Cleanup(func() {
		if !t.Failed() {
			os.RemoveAll(workDir)
		} else {
			t.Logf("Test failed - work directory preserved: %s", workDir)
		}
	})

	t.Logf("Test working directory: %s", workDir)

	// Get absolute path of test directory
	testDirAbs, err := filepath.Abs(testDir)
	require.NoError(t, err)

	// Copy test files to work directory
	CopyDir(t, testDirAbs, workDir)

	// Fix go.mod to use absolute paths in replace directives
	gomodPath := filepath.Join(workDir, "go.mod")
	if data, err := os.ReadFile(gomodPath); err == nil {
		content := string(data)
		// Replace relative path with absolute path
		// The testdata/pgo/go.mod uses ../../../../ (4 levels up from test/e2e/testdata/pgo to orchestrion root)
		content = strings.ReplaceAll(content, "replace github.com/DataDog/orchestrion => ../../../..",
			fmt.Sprintf("replace github.com/DataDog/orchestrion => %s", repoRoot))
		require.NoError(t, os.WriteFile(gomodPath, []byte(content), 0644))
	}

	return workDir
}

// cleanupOldWorkDirs keeps only the N most recent directories for a given test
func cleanupOldWorkDirs(t *testing.T, workBase, testName string, keep int) {
	t.Helper()

	entries, err := os.ReadDir(workBase)
	if err != nil {
		return // Directory doesn't exist yet or can't read it
	}

	// Filter directories matching this test name
	var matchingDirs []os.DirEntry
	prefix := testName + "-"
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			matchingDirs = append(matchingDirs, entry)
		}
	}

	// If we have more than we want to keep, delete the oldest ones
	if len(matchingDirs) <= keep {
		return
	}

	// Sort by name (which includes timestamp, so newest first)
	// Directory names are like: TestPGO-2024-10-16-18-30-45
	type dirInfo struct {
		name string
		path string
	}
	var dirs []dirInfo
	for _, entry := range matchingDirs {
		dirs = append(dirs, dirInfo{
			name: entry.Name(),
			path: filepath.Join(workBase, entry.Name()),
		})
	}

	// Sort by name descending (newest first due to timestamp format)
	for i := 0; i < len(dirs)-1; i++ {
		for j := i + 1; j < len(dirs); j++ {
			if dirs[i].name < dirs[j].name {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}

	// Delete all but the N most recent
	for i := keep; i < len(dirs); i++ {
		os.RemoveAll(dirs[i].path)
	}
}

// WaitForCommandWithTimeout runs a command and waits for it to complete with a timeout
func WaitForCommandWithTimeout(t *testing.T, cmd *exec.Cmd, timeout time.Duration) error {
	t.Helper()

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		cmd.Process.Kill()
		return fmt.Errorf("command timed out after %v", timeout)
	}
}
