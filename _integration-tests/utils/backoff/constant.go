// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package backoff

import "time"

type constantStrategy time.Duration

// NewConstantStrategy returns a constant backoff strategy, waiting for the
// specified delay between each attempt.
func NewConstantStrategy(delay time.Duration) Strategy {
	return constantStrategy(delay)
}

func (c constantStrategy) Next() time.Duration {
	return time.Duration(c)
}
