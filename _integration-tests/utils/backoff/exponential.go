// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package backoff

import (
	"fmt"
	"time"
)

type exponentialStrategy struct {
	next   time.Duration
	factor int
	max    time.Duration
}

// NewExponentialStrategy returns a new exponential back-off strategy that
// starts with the specified [initial] delay, multiplies it by [factor] after
// each attempt, while capping the delay to [max]. Panics if [initial] is
// greater than [max]; or if factor is <=1.
func NewExponentialStrategy(initial time.Duration, factor int, max time.Duration) Strategy {
	if initial > max {
		panic(fmt.Errorf("invalid exponential back-off strategy: initial delay %s is greater than max delay %s", initial, max))
	}
	if factor <= 1 {
		panic(fmt.Errorf("invalid exponential back-off strategy: factor %d must be greater than 0", factor))
	}
	return &exponentialStrategy{next: initial, factor: factor, max: max}
}

func (e *exponentialStrategy) Next() time.Duration {
	defer e.inc()
	return e.next
}

func (e *exponentialStrategy) inc() {
	if e.next == e.max {
		// Capped, we have nothing to do anymore.
		return
	}
	// Multiply the current value by the factor...
	e.next *= time.Duration(e.factor)
	// If we exceeded the cap, truncate to the cap.
	if e.next > e.max {
		e.next = e.max
	}
}
