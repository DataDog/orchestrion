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
	"math"
	"time"
)

type Strategy interface {
	// Next returns the back-off delay to wait for before making the next attempt.
	Next() time.Duration
}

const (
	defaultMaxAttempts = 10
)

// RetryAllErrors is the default function used by [RetryOptions.ShouldRetry]. It
// returns [true] regardless of its arguments.
func RetryAllErrors(error, int, time.Duration) bool {
	return true
}

type RetryOptions struct {
	// MaxAttempts is the maximum number of attempts to make before giving up. If
	// it is negative, there is no limit to the number of attempts (it will be set
	// to [math.MaxInt]); if it is zero, the default value of 10 will be used. It
	// is fine (although a little silly) to set [RetryOptions.MaxAttempts] to 1.
	MaxAttempts int
	// ShouldRetry is called with the error returned by the action, the attempt
	// number, and the delay before the next attempt could be made. If it returns
	// [true], the next attempt will be made; otherwise, the [Retry] function will
	// immediately return. If [nil], the default [RetryAllErrors] function will be
	// used.
	ShouldRetry func(err error, attempt int, delay time.Duration) bool
	// Sleep is the function used to wait in between attempts. It is intended to
	// be used in testing. If [nil], the default [time.Sleep] function will be
	// used.
	Sleep func(time.Duration)
}

// Retry makes up to [RetryOptions.MaxAttempts] at calling the [action]
// function. It uses the [Strategy] to determine how much time to wait between
// attempts. The [RetryOptions.ShouldRetry] function is called with all
// non-[nil] errors returned by [action], the attempt number, and the delay
// before the next attempt. If it returns [true], the [RetryOptions.Sleep]
// function is called with the delay, and the next attempt is made. Otherwise,
// [Retry] returns immediately.
func Retry(
	ctx context.Context,
	strategy Strategy,
	action func() error,
	opts *RetryOptions,
) error {
	var (
		maxAttempts = defaultMaxAttempts
		shouldRetry = RetryAllErrors
		sleep       = time.Sleep
	)
	if opts != nil {
		if opts.MaxAttempts > 0 {
			maxAttempts = opts.MaxAttempts
		} else if opts.MaxAttempts < 0 {
			maxAttempts = math.MaxInt
		}
		if opts.ShouldRetry != nil {
			shouldRetry = opts.ShouldRetry
		}
		if opts.Sleep != nil {
			sleep = opts.Sleep
		}
	}

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
	return errors.Join(errs, ctx.Err())
}
