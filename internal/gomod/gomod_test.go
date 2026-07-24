// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package gomod

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandEnv(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"GOWORK=/some/path/go.work",
		"GOTOOLCHAIN=go1.99",
	}

	env := commandEnv(base)

	// Unrelated variables are preserved.
	assert.Contains(t, env, "PATH=/usr/bin")

	// GOWORK=off is required so that -modfile commands are not rejected in
	// workspace mode ("go: -modfile cannot be used in workspace mode").
	assert.Contains(t, env, "GOWORK=off")
	assert.Contains(t, env, "GOTOOLCHAIN=local")

	// The overrides are appended last so they win over any inherited value.
	last := make(map[string]string)
	for _, e := range env {
		key, val, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		last[key] = val
	}
	assert.Equal(t, "off", last["GOWORK"], "GOWORK=off must win over inherited GOWORK")
	assert.Equal(t, "local", last["GOTOOLCHAIN"], "GOTOOLCHAIN=local must win over inherited GOTOOLCHAIN")
}
