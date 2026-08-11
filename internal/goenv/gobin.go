// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goenv

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var goBinPath string

// GoBinPath returns the resolved path to the `go` command's binary. The result is cached to avoid
// looking it up multiple times. If the lookup fails, the error is returned and the result is not
// cached.
func GoBinPath() (string, error) {
	if goBinPath == "" {
		goBin, err := exec.LookPath("go")
		if err != nil {
			return "", err
		}
		goBinPath = goBin
	}
	return goBinPath, nil
}

// GOVERSION returns the version reported by the resolved `go` command for the
// designated working directory. The directory matters when Go selects a newer
// toolchain based on a module's `go` or `toolchain` directive.
func GOVERSION(dir string) (string, error) {
	goBin, err := GoBinPath()
	if err != nil {
		return "", fmt.Errorf("resolving go binary: %w", err)
	}
	cmd := exec.Command(goBin, "env", "GOVERSION")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if detail := strings.TrimSpace(string(exitErr.Stderr)); detail != "" {
				return "", fmt.Errorf("running %q: %w: %s", []string{goBin, "env", "GOVERSION"}, err, detail)
			}
		}
		return "", fmt.Errorf("running %q: %w", []string{goBin, "env", "GOVERSION"}, err)
	}
	if goVersion := strings.TrimSpace(string(output)); goVersion != "" {
		return goVersion, nil
	}
	return "", fmt.Errorf("running %q returned a blank version", []string{goBin, "env", "GOVERSION"})
}
