// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package backoff

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry(t *testing.T) {
	// The sequence of delays observed for 10 attempts using an exponential
	// backoff strategy with initial delay of 100ms, factor of 2, max delay of 5s.
	delaySequence := []time.Duration{
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
	}

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

		err := RetryVoid(ctx, strategy, action, &RetryOptions{MaxAttempts: maxAttempts, Sleep: timeSleep})
		require.Error(t, err)
		assert.Equal(t, delaySequence, delays)
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

		err := RetryVoid(ctx, strategy, action, &RetryOptions{MaxAttempts: maxAttempts, ShouldRetry: shouldRetry, Sleep: timeSleep})
		require.Error(t, err)
		// We hit the non-retryable error at the 3rd attempt.
		assert.Equal(t, delaySequence[:2], delays)
		for _, expectedErr := range expectedErrs {
			assert.ErrorIs(t, err, expectedErr)
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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

			// Simulate context deadline after 1 second.
			var ttl time.Duration
			for _, delay := range delays {
				ttl += delay
			}
			if ttl >= time.Second {
				cancel()
			}
		}

		err := RetryVoid(ctx, strategy, action, &RetryOptions{MaxAttempts: maxAttempts, Sleep: timeSleep})
		require.Error(t, err)
		// We reach the 1 second total waited during the 4th back-off.
		assert.Equal(t, delaySequence[:4], delays)
		for _, expectedErr := range expectedErrs {
			require.ErrorIs(t, err, expectedErr)
		}
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("unlimited retries", func(t *testing.T) {
		ctx := context.Background()
		strategy := NewConstantStrategy(100 * time.Millisecond)
		var attempts int
		action := func() (int, error) {
			attempts++
			// At least 20 errors, then flip a coin... but no more than 100 attempts.
			if attempts < 20 || (attempts < 100 && rand.Int()%2 == 0) {
				return -1, fmt.Errorf("Error number %d", attempts)
			}
			return attempts, nil
		}
		var delayCount int
		timeSleep := func(time.Duration) {
			delayCount++
		}

		res, err := Retry(ctx, strategy, action, &RetryOptions{MaxAttempts: -1, Sleep: timeSleep})
		require.NoError(t, err)
		assert.Equal(t, attempts, res)
		// We should have waited as many times as we attempted, except for the initial attempt.
		assert.Equal(t, delayCount, attempts-1)
	})

	t.Run("immediate success", func(t *testing.T) {
		ctx := context.Background()
		strategy := NewExponentialStrategy(100*time.Millisecond, 2, 5*time.Second)
		maxAttempts := 10
		shouldRetry := func(error, int, time.Duration) bool { return false }
		action := func() (int, error) { return 1337, nil }
		delays := make([]time.Duration, 0, maxAttempts)
		timeSleep := func(d time.Duration) {
			delays = append(delays, d)
		}

		res, err := Retry(ctx, strategy, action, &RetryOptions{MaxAttempts: maxAttempts, ShouldRetry: shouldRetry, Sleep: timeSleep})
		require.NoError(t, err)
		assert.Equal(t, 1337, res)
		assert.Empty(t, delays)
	})
}
