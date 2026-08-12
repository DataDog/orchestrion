// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScopeInferredTestCoverage(t *testing.T) {
	buildFlags := []string{
		"-race",
		"-cover",
		"-covermode=atomic",
		"-coverpkg=example.com/all/...",
		"-toolexec=orchestrion toolexec",
	}

	assert.Equal(t, []string{
		"-race",
		"-toolexec=orchestrion toolexec",
	}, scopeInferredTestCoverage(buildFlags, "atomic", nil))

	assert.Equal(t, []string{
		"-race",
		"-toolexec=orchestrion toolexec",
		"-cover",
		"-coverpkg=example.com/no-tests,example.com/subject",
		"-covermode=atomic",
	}, scopeInferredTestCoverage(buildFlags, "atomic", []string{
		"example.com/no-tests",
		"example.com/subject",
	}))
}
