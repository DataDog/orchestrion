// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goproxy

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

var values map[string]string = make(map[string]string, 1)

// Goenv obtains the value of a `go env` environment variable. It attempts to read from the
// environment first, and shells out to `go env` in case the environment variable is blank.
func Goenv(name string) (string, error) {
	if value, ok := values[name]; ok {
		return value, nil
	}

	if value := os.Getenv(name); value != "" {
		values[name] = value
		return value, nil
	}

	cmd := exec.Command("go", "env", name)
	stdout := bytes.NewBuffer(make([]byte, 0, 1024))
	cmd.Stdout = stdout

	if err := cmd.Run(); err != nil {
		return "", err
	}

	val := strings.TrimSpace(stdout.String())
	values[name] = val
	return val, nil
}
