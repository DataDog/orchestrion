// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "sort"

// Range is a half-open tainted byte interval in a string or byte slice.
type Range struct {
	Start int
	End   int
}

type addressRange struct {
	start uintptr
	end   uintptr
}

func relativeRanges(ranges []addressRange, start uintptr, length int) []Range {
	if length == 0 {
		return nil
	}
	end := start + uintptr(length)
	result := make([]Range, 0, len(ranges))
	for _, current := range ranges {
		intersectionStart := max(current.start, start)
		intersectionEnd := min(current.end, end)
		if intersectionStart < intersectionEnd {
			result = append(result, Range{
				Start: int(intersectionStart - start),
				End:   int(intersectionEnd - start),
			})
		}
	}
	return normalizeRanges(result, length)
}

func replaceAddressRanges(existing []addressRange, start uintptr, length int, replacement []Range) []addressRange {
	if length == 0 {
		return existing
	}
	end := start + uintptr(length)
	result := make([]addressRange, 0, len(existing)+len(replacement))
	for _, current := range existing {
		if current.end <= start || current.start >= end {
			result = append(result, current)
			continue
		}
		if current.start < start {
			result = append(result, addressRange{start: current.start, end: start})
		}
		if current.end > end {
			result = append(result, addressRange{start: end, end: current.end})
		}
	}
	for _, current := range normalizeRanges(replacement, length) {
		result = append(result, addressRange{
			start: start + uintptr(current.Start),
			end:   start + uintptr(current.End),
		})
	}
	return normalizeAddressRanges(result)
}

func normalizeRanges(ranges []Range, length int) []Range {
	result := make([]Range, 0, len(ranges))
	for _, current := range ranges {
		current.Start = max(current.Start, 0)
		current.End = min(current.End, length)
		if current.Start < current.End {
			result = append(result, current)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Start == result[right].Start {
			return result[left].End < result[right].End
		}
		return result[left].Start < result[right].Start
	})
	merged := result[:0]
	for _, current := range result {
		last := len(merged) - 1
		if last >= 0 && current.Start <= merged[last].End {
			merged[last].End = max(merged[last].End, current.End)
			continue
		}
		merged = append(merged, current)
	}
	return merged
}

func normalizeAddressRanges(ranges []addressRange) []addressRange {
	sort.Slice(ranges, func(left, right int) bool {
		if ranges[left].start == ranges[right].start {
			return ranges[left].end < ranges[right].end
		}
		return ranges[left].start < ranges[right].start
	})
	merged := ranges[:0]
	for _, current := range ranges {
		last := len(merged) - 1
		if last >= 0 && current.start <= merged[last].end {
			merged[last].end = max(merged[last].end, current.end)
			continue
		}
		merged = append(merged, current)
	}
	return merged
}

func shiftedRanges(ranges []Range, offset int) []Range {
	result := make([]Range, len(ranges))
	for index, current := range ranges {
		result[index] = Range{Start: current.Start + offset, End: current.End + offset}
	}
	return result
}
