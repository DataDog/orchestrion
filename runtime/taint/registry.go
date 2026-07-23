// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"strings"
	"sync"
	"unsafe"
)

type builderState struct {
	length int
	ranges []Range
}

type registry struct {
	mu sync.RWMutex

	stringOwners []string
	stringRanges []addressRange
	byteOwners   []any
	byteRanges   []addressRange
	runeOwners   []any
	runeRanges   []addressRange
	byteScalars  map[uintptr]struct{}
	runeScalars  map[uintptr]struct{}
	builders     map[*strings.Builder]builderState
}

const maxTrackedOccurrences = 1 << 16

var active = registry{
	byteScalars: make(map[uintptr]struct{}),
	runeScalars: make(map[uintptr]struct{}),
	builders:    make(map[*strings.Builder]builderState),
}

// RangesString returns the tainted byte ranges visible through value.
func RangesString(value string) []Range {
	start, ok := stringAddress(value)
	if !ok {
		return nil
	}
	active.mu.RLock()
	defer active.mu.RUnlock()
	return relativeRanges(active.stringRanges, start, len(value))
}

// RangesBytes returns the tainted byte ranges visible through value.
func RangesBytes[T ~[]E, E ~byte](value T) []Range {
	start, ok := bytesAddress(value)
	if !ok {
		return nil
	}
	active.mu.RLock()
	defer active.mu.RUnlock()
	return relativeRanges(active.byteRanges, start, len(value))
}

// RangesRunes returns the tainted rune-index ranges visible through value.
func RangesRunes[T ~[]E, E ~rune](value T) []Range {
	if len(value) == 0 {
		return nil
	}
	start := uintptr(unsafe.Pointer(unsafe.SliceData(value)))
	active.mu.RLock()
	defer active.mu.RUnlock()
	byteRanges := relativeRanges(active.runeRanges, start, len(value)*4)
	result := make([]Range, len(byteRanges))
	for index, current := range byteRanges {
		result[index] = Range{Start: current.Start / 4, End: current.End / 4}
	}
	return result
}

func registerString(value string, ranges []Range) {
	start, ok := stringAddress(value)
	if !ok {
		return
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	registerStringLocked(value, start, ranges)
}

func registerStringLocked(value string, start uintptr, ranges []Range) {
	if len(ranges) > 0 && len(active.stringOwners) >= maxTrackedOccurrences {
		return
	}
	active.stringRanges = replaceAddressRanges(active.stringRanges, start, len(value), ranges)
	if len(ranges) > 0 {
		active.stringOwners = append(active.stringOwners, value)
	}
}

func registerBytes[T ~[]E, E ~byte](value T, ranges []Range) {
	start, ok := bytesAddress(value)
	if !ok {
		return
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	if len(ranges) > 0 && len(active.byteOwners) >= maxTrackedOccurrences {
		return
	}
	active.byteRanges = replaceAddressRanges(active.byteRanges, start, len(value), ranges)
	if len(ranges) > 0 {
		active.byteOwners = append(active.byteOwners, value)
	}
}

func registerRunes[T ~[]E, E ~rune](value T, ranges []Range) {
	if len(value) == 0 {
		return
	}
	start := uintptr(unsafe.Pointer(unsafe.SliceData(value)))
	scaled := make([]Range, len(ranges))
	for index, current := range ranges {
		scaled[index] = Range{Start: current.Start * 4, End: current.End * 4}
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	if len(ranges) > 0 && len(active.runeOwners) >= maxTrackedOccurrences {
		return
	}
	active.runeRanges = replaceAddressRanges(active.runeRanges, start, len(value)*4, scaled)
	if len(ranges) > 0 {
		active.runeOwners = append(active.runeOwners, value)
	}
}

func stringAddress(value string) (uintptr, bool) {
	if len(value) == 0 {
		return 0, false
	}
	return uintptr(unsafe.Pointer(unsafe.StringData(value))), true
}

func bytesAddress[T ~[]E, E ~byte](value T) (uintptr, bool) {
	if len(value) == 0 {
		return 0, false
	}
	return uintptr(unsafe.Pointer(unsafe.SliceData(value))), true
}

func isolatedString(value string) string {
	if len(value) == 0 {
		return ""
	}
	buffer := make([]byte, len(value)+1)
	copy(buffer, value)
	withSentinel := string(buffer)
	return withSentinel[:len(value)]
}
