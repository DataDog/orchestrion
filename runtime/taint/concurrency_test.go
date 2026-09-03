// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"fmt"
	"sync"
	"testing"
)

func Test_ConcurrentPropagationIsIsolated_when_GoroutinesTrackDistinctValues(t *testing.T) {
	// Given
	const goroutines = 32
	var wait sync.WaitGroup
	errors := make(chan string, goroutines)

	// When
	for index := range goroutines {
		wait.Go(func() {
			value := ConcatStrings("prefix-", SourceString(fmt.Sprintf("value-%d", index)))
			ranges := RangesString(value)
			if len(ranges) != 1 || ranges[0].Start != 7 || ranges[0].End != len(value) {
				errors <- fmt.Sprintf("value %q ranges %#v", value, ranges)
			}
		})
	}
	wait.Wait()
	close(errors)

	// Then
	for message := range errors {
		t.Error(message)
	}
}
