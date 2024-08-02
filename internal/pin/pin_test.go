// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/datadog/orchestrion/internal/version"
	"github.com/stretchr/testify/require"
)

func TestPin(t *testing.T) {
	tmp := t.TempDir()
	if cwd, err := os.Getwd(); err == nil {
		defer require.NoError(t, os.Chdir(cwd))
	}

	require.NoError(t, scaffold(tmp))
	require.NoError(t, os.Chdir(tmp))
	requiredVersionError = errors.New("test")
	AutoPinOrchestrion()
	require.NoError(t, requiredVersionError)

	require.FileExists(t, filepath.Join(tmp, orchestrionToolGo))
	require.FileExists(t, filepath.Join(tmp, "go.sum"))

	data, err := os.ReadFile(filepath.Join(tmp, "go.mod"))
	require.NoError(t, err)
	require.Contains(t, string(data), fmt.Sprintf(`github.com/datadog/orchestrion %s`, version.Tag))
}

func scaffold(dir string) error {
	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(thisFile, "..", "..", "..")

	goMod, err := os.Create(filepath.Join(dir, "go.mod"))
	if err != nil {
		return err
	}
	defer goMod.Close()

	if _, err := fmt.Fprintln(goMod, "module github.com/datadog/orchestrion/pin-test"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(goMod); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(goMod, "go %s\n", runtime.Version()[2:6]); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(goMod); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(goMod, "replace github.com/datadog/orchestrion %s => %s\n", version.Tag, rootDir); err != nil {
		return err
	}

	return nil
}
