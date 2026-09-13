// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "sort"

// Range is a half-open tainted byte interval in a string or byte slice.
type Range struct {
	Start    int
	End      int
	SourceID uint64 `json:"source_id,omitempty"`
}

type addressRange struct {
	start    uintptr
	end      uintptr
	sourceID uint64
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
				Start:    int(intersectionStart - start),
				End:      int(intersectionEnd - start),
				SourceID: current.sourceID,
			})
		}
	}
	return normalizeRanges(result, length)
}

func replaceAddressRanges(existing []addressRange, maximumEnd, start uintptr, length int, replacement []Range) ([]addressRange, uintptr) {
	if length == 0 {
		return existing, maximumEnd
	}
	normalized := normalizeRanges(replacement, length)
	if len(normalized) > 0 && start > maximumEnd {
		for _, current := range normalized {
			mapped := addressRange{
				start:    start + uintptr(current.Start),
				end:      start + uintptr(current.End),
				sourceID: current.SourceID,
			}
			existing = append(existing, mapped)
			maximumEnd = max(maximumEnd, mapped.end)
		}
		return existing, maximumEnd
	}
	end := start + uintptr(length)
	result := make([]addressRange, 0, len(existing)+len(replacement))
	for _, current := range existing {
		if current.end <= start || current.start >= end {
			result = append(result, current)
			continue
		}
		if current.start < start {
			result = append(result, addressRange{start: current.start, end: start, sourceID: current.sourceID})
		}
		if current.end > end {
			result = append(result, addressRange{start: end, end: current.end, sourceID: current.sourceID})
		}
	}
	for _, current := range normalized {
		result = append(result, addressRange{
			start:    start + uintptr(current.Start),
			end:      start + uintptr(current.End),
			sourceID: current.SourceID,
		})
	}
	result = normalizeAddressRanges(result)
	maximumEnd = 0
	for _, current := range result {
		maximumEnd = max(maximumEnd, current.end)
	}
	return result, maximumEnd
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
		if last >= 0 && current.SourceID == merged[last].SourceID && current.Start <= merged[last].End {
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
		if last >= 0 && current.sourceID == merged[last].sourceID && current.start <= merged[last].end {
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
		result[index] = Range{Start: current.Start + offset, End: current.End + offset, SourceID: current.SourceID}
	}
	return result
}

func conservativeRanges(length int, input []Range) []Range {
	if length == 0 || len(input) == 0 {
		return nil
	}
	ids := make(map[uint64]struct{}, len(input))
	for _, current := range input {
		if current.SourceID != 0 {
			ids[current.SourceID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return []Range{{Start: 0, End: length}}
	}
	sorted := make([]uint64, 0, len(ids))
	for sourceID := range ids {
		sorted = append(sorted, sourceID)
	}
	sort.Slice(sorted, func(left, right int) bool { return sorted[left] < sorted[right] })
	result := make([]Range, 0, len(sorted))
	for _, sourceID := range sorted {
		result = append(result, Range{Start: 0, End: length, SourceID: sourceID})
	}
	return result
}
