// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "strings"

// BuilderWriteString writes value and records its ranges at the written offset.
func BuilderWriteString(builder *strings.Builder, value string) (int, error) {
	ranges := RangesString(value)
	before := builder.Len()
	written, err := builder.WriteString(value)

	active.mu.Lock()
	defer active.mu.Unlock()
	state, tracked := active.builders[builder]
	if !tracked && len(ranges) == 0 {
		return written, err
	}
	if !tracked && len(active.builders) >= maxTrackedOccurrences {
		return written, err
	}
	if state.length > before {
		state = builderState{}
	}
	state.ranges = append(state.ranges, shiftedRanges(normalizeRanges(ranges, written), before)...)
	state.ranges = normalizeRanges(state.ranges, builder.Len())
	state.length = builder.Len()
	if len(state.ranges) == 0 {
		delete(active.builders, builder)
	} else {
		active.builders[builder] = state
	}
	return written, err
}

// BuilderString returns the current builder contents and attaches a metadata snapshot.
func BuilderString(builder *strings.Builder) string {
	result := builder.String()
	start, ok := stringAddress(result)
	if !ok {
		return result
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	state := active.builders[builder]
	registerStringLocked(result, start, state.ranges)
	return result
}
