// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

type readerState struct {
	length int
	offset int
	ranges []Range
}

// NewStringReader tracks string ranges for a new reader.
func NewStringReader(value string) *strings.Reader {
	reader := strings.NewReader(value)
	ranges := RangesString(value)
	if len(ranges) == 0 {
		return reader
	}

	registry := active.currentRegistry()
	registry.mu.Lock()
	if len(registry.stringReaders) < maxTrackedOccurrences {
		registry.stringReaders[reader] = readerState{length: len(value), ranges: ranges}
	} else {
		registry.stateSaturated = true
	}
	registry.mu.Unlock()
	return reader
}

// NewBufioReader transfers tracked string-reader state to a buffered reader.
func NewBufioReader(source io.Reader) *bufio.Reader {
	reader := bufio.NewReader(source)
	stringReader, ok := source.(*strings.Reader)
	if !ok {
		return reader
	}

	registry := active.registryForStringReader(stringReader)
	registry.mu.Lock()
	state, tracked := registry.stringReaders[stringReader]
	if tracked {
		delete(registry.stringReaders, stringReader)
		if len(registry.bufioReaders) < maxTrackedOccurrences {
			registry.bufioReaders[reader] = state
		} else {
			registry.stateSaturated = true
		}
	}
	registry.mu.Unlock()
	return reader
}

// BufioReadString reads through delimiter and projects tracked source ranges.
func BufioReadString(reader *bufio.Reader, delimiter byte) (string, error) {
	result, err := reader.ReadString(delimiter)

	registry := active.registryForBufioReader(reader)
	registry.mu.Lock()
	state, tracked := registry.bufioReaders[reader]
	var ranges []Range
	if tracked {
		ranges = projectRanges(state.ranges, state.offset, state.offset+len(result), 0)
		state.offset += len(result)
		if state.offset >= state.length {
			delete(registry.bufioReaders, reader)
		} else {
			registry.bufioReaders[reader] = state
		}
	}
	registry.mu.Unlock()

	if tracked {
		registerString(result, ranges)
	}
	return result, err
}

// IOCopy copies data and projects ranges for tracked string readers copied into buffers.
func IOCopy(destination io.Writer, source io.Reader) (int64, error) {
	buffer, bufferDestination := destination.(*bytes.Buffer)
	reader, stringSource := source.(*strings.Reader)
	if !bufferDestination || !stringSource {
		return io.Copy(destination, source)
	}

	registry := active.registryForStringReader(reader)
	registry.mu.RLock()
	state, tracked := registry.stringReaders[reader]
	registry.mu.RUnlock()
	if !tracked {
		return io.Copy(destination, source)
	}

	destinationRanges := RangesBytes(buffer.Bytes())
	destinationLength := buffer.Len()
	written, err := io.Copy(destination, source)
	sourceRanges := projectRanges(state.ranges, state.offset, state.offset+int(written), 0)

	registry.mu.Lock()
	if _, exists := registry.stringReaders[reader]; exists {
		state.offset += int(written)
		if state.offset >= state.length {
			delete(registry.stringReaders, reader)
		} else {
			registry.stringReaders[reader] = state
		}
	}
	registry.mu.Unlock()

	ranges := append(destinationRanges, shiftedRanges(sourceRanges, destinationLength)...)
	registerBytes(buffer.Bytes(), ranges)
	return written, err
}
