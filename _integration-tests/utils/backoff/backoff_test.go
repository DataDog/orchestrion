// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package backoff

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry(t *testing.T) {
	t.Run("no-success", func(t *testing.T) {
		ctx := context.Background()
		strategy := NewExponentialStrategy(100*time.Millisecond, 2, 5*time.Second)
		maxAttempts := 10
		expectedErrs := make([]error, 0, maxAttempts)
		action := func() error {
			err := fmt.Errorf("Error number %d", len(expectedErrs)+1)
			expectedErrs = append(expectedErrs, err)
			return err
		}
		delays := make([]time.Duration, 0, maxAttempts)
		timeSleep := func(d time.Duration) {
			delays = append(delays, d)
		}

		err := doRetry(ctx, strategy, maxAttempts, nil, action, timeSleep)
		require.Error(t, err)
		assert.Equal(t,
			[]time.Duration{
				/* attempt  1 */ // Immediate
				/* attempt  2 */ 100 * time.Millisecond,
				/* attempt  3 */ 200 * time.Millisecond,
				/* attempt  4 */ 400 * time.Millisecond,
				/* attempt  5 */ 800 * time.Millisecond,
				/* attempt  6 */ 1600 * time.Millisecond,
				/* attempt  7 */ 3200 * time.Millisecond,
				/* attempt  8 */ 5 * time.Second,
				/* attempt  9 */ 5 * time.Second,
				/* attempt 10 */ 5 * time.Second,
			},
			delays,
		)
		for _, expectedErr := range expectedErrs {
			assert.ErrorIs(t, err, expectedErr)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		ctx := context.Background()
		strategy := NewExponentialStrategy(100*time.Millisecond, 2, 5*time.Second)
		maxAttempts := 10
		shouldRetry := func(err error, _ int, _ time.Duration) bool {
			return !strings.Contains(err.Error(), "3")
		}
		expectedErrs := make([]error, 0, maxAttempts)
		action := func() error {
			err := fmt.Errorf("Error number %d", len(expectedErrs)+1)
			expectedErrs = append(expectedErrs, err)
			return err
		}
		delays := make([]time.Duration, 0, maxAttempts)
		timeSleep := func(d time.Duration) {
			delays = append(delays, d)
		}

		err := doRetry(ctx, strategy, maxAttempts, shouldRetry, action, timeSleep)
		require.Error(t, err)
		assert.Equal(t,
			[]time.Duration{
				/* attempt  1 */ // Immediate
				/* attempt  2 */ 100 * time.Millisecond,
				/* attempt  3 */ 200 * time.Millisecond,
			},
			delays,
		)
		for _, expectedErr := range expectedErrs {
			assert.ErrorIs(t, err, expectedErr)
		}
	})

	t.Run("immediate success", func(t *testing.T) {
		ctx := context.Background()
		strategy := NewExponentialStrategy(100*time.Millisecond, 2, 5*time.Second)
		maxAttempts := 10
		shouldRetry := func(err error, _ int, _ time.Duration) bool { return false }
		action := func() error { return nil }
		delays := make([]time.Duration, 0, maxAttempts)
		timeSleep := func(d time.Duration) {
			delays = append(delays, d)
		}

		err := doRetry(ctx, strategy, maxAttempts, shouldRetry, action, timeSleep)
		require.NoError(t, err)
		assert.Empty(t, delays)
	})
}
