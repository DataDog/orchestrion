// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package built_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	tmp := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmp, "main.go"), []byte(testProgram), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo), []byte(orchestrionToolGo), 0o644))

	cmd := exec.Command("go", "mod", "init", "dummy.test")
	cmd.Dir = tmp
	require.NoError(t, cmd.Run())

	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(thisFile, "..", "..", "..")
	cmd = exec.Command("go", "mod", "edit", "-replace=github.com/DataDog/orchestrion="+rootDir)
	cmd.Dir = tmp
	require.NoError(t, cmd.Run())

	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = tmp
	require.NoError(t, cmd.Run())

	var stdout bytes.Buffer
	cmd = exec.Command("go", "run", "github.com/DataDog/orchestrion", "go", "run", ".")
	cmd.Dir = tmp
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	require.Equal(t, version.Tag(), stdout.String())
}

const testProgram = `package main

import (
	"fmt"
	"os"

	"github.com/DataDog/orchestrion/runtime/built"
)

func main() {
	if !built.WithOrchestrion {
		os.Exit(42)
	}

	fmt.Print(built.WithOrchestrionVersion)
}
`

const orchestrionToolGo = `//go:build tools

package tools

import (
	_ "github.com/DataDog/orchestrion"
)
`
