// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goenv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGOMOD(t *testing.T) {
	t.Run("without GOMOD environment variable", func(t *testing.T) {
		gomod, err := GOMOD("")
		require.NoError(t, err)
		require.NotEmpty(t, gomod)
	})

	t.Run("no GOMOD can be found", func(t *testing.T) {
		val, err := GOMOD(t.TempDir())
		require.Empty(t, val)
		require.ErrorIs(t, err, ErrNoGoMod)
	})
}
