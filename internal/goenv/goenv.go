// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goenv

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	// ErrNoGoMod is returned when no GOMOD value could be identified.
	ErrNoGoMod = errors.New("`go mod GOMOD` returned a blank string")
)

// GOMOD returns the current GOMOD environment variable (possibly from running `go env GOMOD`).
func GOMOD() (string, error) {
	if goMod := os.Getenv("GOMOD"); goMod != "" {
		return goMod, nil
	}
	cmd := exec.Command("go", "env", "GOMOD")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("runnning %q: %w", cmd.Args, err)
	}
	if goMod := strings.TrimSpace(stdout.String()); goMod != "" {
		return goMod, nil
	}

	wd, _ := os.Getwd()
	return "", fmt.Errorf("in %q: %w", wd, ErrNoGoMod)
}
