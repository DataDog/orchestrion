// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package backoff

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExponentialStrategy(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		// Invalid factor (must be >= 2)
		require.Panics(t, func() { NewExponentialStrategy(time.Second, -1, time.Minute) })
		require.Panics(t, func() { NewExponentialStrategy(time.Second, 0, time.Minute) })
		require.Panics(t, func() { NewExponentialStrategy(time.Second, 1, time.Minute) })
		// Invalid initial/cap (initial must be <= cap)
		require.Panics(t, func() { NewExponentialStrategy(time.Minute, 2, time.Second) })
	})
}
