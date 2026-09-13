// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"bufio"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

type builderState struct {
	length int
	ranges []Range
}

type registry struct {
	mu sync.RWMutex

	stringOwners   map[uintptr]string
	stringRanges   []addressRange
	stringRangeEnd uintptr
	byteOwners     map[uintptr]any
	byteRanges     []addressRange
	byteRangeEnd   uintptr
	runeOwners     map[uintptr]any
	runeRanges     []addressRange
	runeRangeEnd   uintptr
	byteScalars    map[unsafe.Pointer]struct{}
	runeScalars    map[unsafe.Pointer]struct{}
	builders       map[*strings.Builder]builderState
	stringReaders  map[*strings.Reader]readerState
	bufioReaders   map[*bufio.Reader]readerState
	stateSaturated bool
}

const maxTrackedOccurrences = 1 << 16

func newRegistry() registry {
	return registry{
		stringOwners:  make(map[uintptr]string),
		byteOwners:    make(map[uintptr]any),
		runeOwners:    make(map[uintptr]any),
		byteScalars:   make(map[unsafe.Pointer]struct{}),
		runeScalars:   make(map[unsafe.Pointer]struct{}),
		builders:      make(map[*strings.Builder]builderState),
		stringReaders: make(map[*strings.Reader]readerState),
		bufioReaders:  make(map[*bufio.Reader]readerState),
	}
}

var active = newRegistryLifecycle()

var sourceIDs atomic.Uint64

func newSourceID() uint64 {
	return sourceIDs.Add(1)
}

// RangesString returns the tainted byte ranges visible through value.
func RangesString(value string) []Range {
	ranges, _ := active.stringRangesAndSaturation(value)
	return ranges
}

// RangesBytes returns the tainted byte ranges visible through value.
func RangesBytes[T ~[]E, E ~byte](value T) []Range {
	start, ok := bytesAddress(value)
	if !ok {
		return nil
	}
	return active.byteRanges(start, len(value))
}

// RangesRunes returns the tainted rune-index ranges visible through value.
func RangesRunes[T ~[]E, E ~rune](value T) []Range {
	if len(value) == 0 {
		return nil
	}
	start := uintptr(unsafe.Pointer(unsafe.SliceData(value)))
	byteRanges := active.runeRanges(start, len(value)*4)
	result := make([]Range, len(byteRanges))
	for index, current := range byteRanges {
		result[index] = Range{Start: current.Start / 4, End: current.End / 4, SourceID: current.SourceID}
	}
	return result
}

func registerString(value string, ranges []Range) {
	active.currentRegistry().registerString(value, ranges)
}

func registerFreshString(value string, sourceID uint64) {
	active.currentRegistry().registerFreshString(value, sourceID)
}

func (registry *registry) registerString(value string, ranges []Range) {
	start, ok := stringAddress(value)
	if !ok {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.registerStringLocked(value, start, ranges)
}

func (registry *registry) registerFreshString(value string, sourceID uint64) {
	start, ok := stringAddress(value)
	if !ok {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	end := start + uintptr(len(value))
	registry.stringRanges = append(registry.stringRanges, addressRange{start: start, end: end, sourceID: sourceID})
	registry.stringRangeEnd = max(registry.stringRangeEnd, end)
	registry.stringOwners[start] = value
}

func (registry *registry) registerStringLocked(value string, start uintptr, ranges []Range) {
	registry.stringRanges, registry.stringRangeEnd = replaceAddressRanges(registry.stringRanges, registry.stringRangeEnd, start, len(value), ranges)
	if len(ranges) > 0 {
		registry.stringOwners[start] = value
	}
}

func registerBytes[T ~[]E, E ~byte](value T, ranges []Range) {
	start, ok := bytesAddress(value)
	if !ok {
		return
	}
	registry := active.currentRegistry()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.registerBytesLocked(start, len(value), value, ranges)
}

func (registry *registry) registerBytesLocked(start uintptr, length int, owner any, ranges []Range) {
	registry.byteRanges, registry.byteRangeEnd = replaceAddressRanges(registry.byteRanges, registry.byteRangeEnd, start, length, ranges)
	if len(ranges) > 0 {
		registry.byteOwners[start] = owner
	}
}

func registerRunes[T ~[]E, E ~rune](value T, ranges []Range) {
	if len(value) == 0 {
		return
	}
	start := uintptr(unsafe.Pointer(unsafe.SliceData(value)))
	scaled := make([]Range, len(ranges))
	for index, current := range ranges {
		scaled[index] = Range{Start: current.Start * 4, End: current.End * 4, SourceID: current.SourceID}
	}
	registry := active.currentRegistry()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.registerRunesLocked(start, len(value)*4, value, scaled)
}

func (registry *registry) registerRunesLocked(start uintptr, length int, owner any, ranges []Range) {
	registry.runeRanges, registry.runeRangeEnd = replaceAddressRanges(registry.runeRanges, registry.runeRangeEnd, start, length, ranges)
	if len(ranges) > 0 {
		registry.runeOwners[start] = owner
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
