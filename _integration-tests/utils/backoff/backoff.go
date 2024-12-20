// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package backoff provides utilities to retry operations when encoutering
// transient errors. It is used by integration tests to allow for testcontainers
// enough time to become fully ready, avoiding tests flaking because the CI
// resources are constrained enough to cause containers to not be "immediately"
// ready to serve traffic.
package backoff

import (
	"context"
	"errors"
	"time"
)

type Strategy interface {
	// Next returns the back-off delay to wait for before making the next attempt.
	Next() time.Duration
}

// Retry makes up to [maxAttempts] at calling the [action] function. It uses the
// [strategy] to determine how much time to wait between attempts. The
// [shouldRetry] functionis called with all non-[nil] errors returned by
// [action] and the retry delay before the next attempt, and should return
// [true] if the error is transient and should be retried, [false] if [Retry]
// should return immediately. If [shouldRetry] is [nil], all errors are retried.
func Retry(
	ctx context.Context,
	strategy Strategy,
	maxAttempts int,
	shouldRetry func(error, int, time.Duration) bool,
	action func() error,
) error {
	return doRetry(ctx, strategy, maxAttempts, shouldRetry, action, time.Sleep)
}

func doRetry(
	ctx context.Context,
	strategy Strategy,
	maxAttempts int,
	shouldRetry func(error, int, time.Duration) bool,
	action func() error,
	sleep func(time.Duration),
) error {
	var errs error

	for attempt, delay := 0, time.Duration(0); attempt < maxAttempts && ctx.Err() == nil; attempt, delay = attempt+1, strategy.Next() {
		if delay > 0 {
			sleep(delay)
		}

		err := action()
		if err == nil {
			// Success!
			return nil
		}

		// Accumulate this error on top of the others we have observed so far.
		errs = errors.Join(errs, err)

		if shouldRetry != nil && !shouldRetry(err, attempt, delay) {
			break
		}
	}

	return errs
}
