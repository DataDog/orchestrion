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
	updateBuilderRanges(builder, ranges, before, written)
	return written, err
}

// BuilderWrite writes a byte slice and records its ranges at the written offset.
func BuilderWrite[B ~[]E, E ~byte](builder *strings.Builder, value B) (int, error) {
	ranges := RangesBytes(value)
	before := builder.Len()
	bytes := make([]byte, len(value))
	for index := range value {
		bytes[index] = byte(value[index])
	}
	written, err := builder.Write(bytes)
	updateBuilderRanges(builder, ranges, before, written)
	return written, err
}

func updateBuilderRanges(builder *strings.Builder, ranges []Range, before, written int) {
	registry := active.registryForBuilder(builder)
	registry.mu.Lock()
	defer registry.mu.Unlock()
	state, tracked := registry.builders[builder]
	if !tracked && len(ranges) == 0 {
		return
	}
	if !tracked && len(registry.builders) >= maxTrackedOccurrences {
		registry.stateSaturated = true
		return
	}
	if state.length > before {
		state = builderState{}
	}
	state.ranges = append(state.ranges, shiftedRanges(normalizeRanges(ranges, written), before)...)
	state.ranges = normalizeRanges(state.ranges, builder.Len())
	state.length = builder.Len()
	if len(state.ranges) == 0 {
		delete(registry.builders, builder)
	} else {
		registry.builders[builder] = state
	}
}

// BuilderString returns the current builder contents and attaches a metadata snapshot.
func BuilderString(builder *strings.Builder) string {
	result := builder.String()
	registry := active.registryForBuilder(builder)
	registry.mu.RLock()
	state := registry.builders[builder]
	registry.mu.RUnlock()
	registerString(result, state.ranges)
	return result
}
