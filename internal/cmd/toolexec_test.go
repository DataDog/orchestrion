// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestJoinCommandCloseErrorPreservesExitCode(t *testing.T) {
	result := cli.Exit("command failed", -1)
	closeErr := errors.New("close failed")

	combined := joinCommandCloseError(result, closeErr)
	exitCoder, ok := combined.(cli.ExitCoder)
	require.True(t, ok)
	assert.Equal(t, -1, exitCoder.ExitCode())
	assert.ErrorIs(t, combined, closeErr)
}
