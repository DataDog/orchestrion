// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"bufio"
	"strings"
	"testing"
)

func Test_OpenFileReportsTaintedPath_when_PathContainsSourceBytes(t *testing.T) {
	// Given
	reports := make(chan Report, 1)
	restore := SetReporter(func(report Report) { reports <- report })
	t.Cleanup(restore)
	path := ConcatStrings("/missing/", SourceString("secret"))

	// When
	file, err := OpenFile(path)
	if file != nil {
		_ = file.Close()
	}

	// Then
	if err == nil {
		t.Fatal("OpenFile() error = nil, want missing-file error")
	}
	select {
	case report := <-reports:
		if report.Sink != "os.Open" || report.Value != path {
			t.Fatalf("report = %#v", report)
		}
		requireRanges(t, report.Ranges, Range{Start: 9, End: 15})
	default:
		t.Fatal("OpenFile() emitted no report")
	}
}

func Test_OpenFileDoesNotReport_when_PathIsClean(t *testing.T) {
	// Given
	reports := make(chan Report, 1)
	restore := SetReporter(func(report Report) { reports <- report })
	t.Cleanup(restore)

	// When
	file, _ := OpenFile("/missing/clean")
	if file != nil {
		_ = file.Close()
	}

	// Then
	select {
	case report := <-reports:
		t.Fatalf("OpenFile() emitted unexpected report %#v", report)
	default:
	}
}

func Test_OpenFileReportsExactRanges_when_StringOwnersExceed65536(t *testing.T) {
	// Given
	active = newRegistryLifecycle()
	t.Cleanup(func() { active = newRegistryLifecycle() })
	reports := make(chan Report, 1)
	restore := SetReporter(func(report Report) { reports <- report })
	t.Cleanup(restore)
	seedStringOwners(t, active.currentRegistry())
	path := SourceString("/missing/overflow")

	// When
	file, err := OpenFile(path)
	if file != nil {
		_ = file.Close()
	}

	// Then
	if err == nil {
		t.Fatal("OpenFile() error = nil, want missing-file error")
	}
	select {
	case report := <-reports:
		if report.Sink != "os.Open" || report.Value != path {
			t.Fatalf("report = %#v, want exact-range report for %q", report, path)
		}
		requireRanges(t, report.Ranges, Range{Start: 0, End: len(path)})
	default:
		t.Fatal("OpenFile() emitted no exact-range report")
	}
}

func Test_OpenFileDoesNotReport_when_CleanPathMatchesTrackedValuePast65536(t *testing.T) {
	// Given
	active = newRegistryLifecycle()
	t.Cleanup(func() { active = newRegistryLifecycle() })
	reports := make(chan Report, 1)
	restore := SetReporter(func(report Report) { reports <- report })
	t.Cleanup(restore)
	seedStringOwners(t, active.currentRegistry())
	tracked := SourceString("/missing/overflow")
	path := isolatedString(tracked)
	if path != tracked {
		t.Fatalf("clean path = %q, want %q", path, tracked)
	}

	// When
	file, _ := OpenFile(path)
	if file != nil {
		_ = file.Close()
	}

	// Then
	select {
	case report := <-reports:
		t.Fatalf("OpenFile() emitted unexpected report %#v", report)
	default:
	}
}

func Test_OpenFileReportsUnknown_when_StateRegistrySaturates(t *testing.T) {
	tests := []struct {
		name     string
		seed     func(t *testing.T)
		overflow func(value string) string
	}{
		{
			name: "builder",
			seed: seedBuilders,
			overflow: func(value string) string {
				builder := new(strings.Builder)
				_, _ = BuilderWriteString(builder, SourceString(value))
				return BuilderString(builder)
			},
		},
		{
			name: "string reader",
			seed: seedStringReaders,
			overflow: func(value string) string {
				reader := NewBufioReader(NewStringReader(SourceString(value)))
				result, _ := BufioReadString(reader, '\n')
				return result
			},
		},
		{
			name: "bufio reader",
			seed: seedBufioReaders,
			overflow: func(value string) string {
				reader := NewBufioReader(NewStringReader(SourceString(value)))
				result, _ := BufioReadString(reader, '\n')
				return result
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			active = newRegistryLifecycle()
			t.Cleanup(func() { active = newRegistryLifecycle() })
			reports := make(chan Report, 1)
			restore := SetReporter(func(report Report) { reports <- report })
			t.Cleanup(restore)
			value := "/missing/overflow"
			clean := isolatedString(value)
			file, _ := OpenFile(clean)
			if file != nil {
				_ = file.Close()
			}
			select {
			case report := <-reports:
				t.Fatalf("OpenFile() emitted clean-control report %#v", report)
			default:
			}
			test.seed(t)

			// When
			path := test.overflow(value)
			file, err := OpenFile(path)
			if file != nil {
				_ = file.Close()
			}

			// Then
			if err == nil {
				t.Fatal("OpenFile() error = nil, want missing-file error")
			}
			select {
			case report := <-reports:
				if report.Sink != "os.Open" || report.Value != value || report.State != Unknown || len(report.Ranges) != 0 {
					t.Fatalf("report = %#v, want unknown report without ranges", report)
				}
			default:
				t.Fatal("OpenFile() emitted no unknown report")
			}
		})
	}
}

func Test_OpenFileReportsExactRanges_when_StateRegistrySaturates(t *testing.T) {
	// Given
	active = newRegistryLifecycle()
	t.Cleanup(func() { active = newRegistryLifecycle() })
	reports := make(chan Report, 1)
	restore := SetReporter(func(report Report) { reports <- report })
	t.Cleanup(restore)
	value := "/missing/overflow"
	clean := isolatedString(value)
	file, _ := OpenFile(clean)
	if file != nil {
		_ = file.Close()
	}
	select {
	case report := <-reports:
		t.Fatalf("OpenFile() emitted clean-control report %#v", report)
	default:
	}
	seedBuilders(t)
	builder := new(strings.Builder)
	_, _ = BuilderWriteString(builder, SourceString(value))
	path := SourceString(value)

	// When
	file, err := OpenFile(path)
	if file != nil {
		_ = file.Close()
	}

	// Then
	if err == nil {
		t.Fatal("OpenFile() error = nil, want missing-file error")
	}
	select {
	case report := <-reports:
		if report.Sink != "os.Open" || report.Value != value || report.State != "" {
			t.Fatalf("report = %#v, want exact-range report", report)
		}
		requireRanges(t, report.Ranges, Range{Start: 0, End: len(value)})
	default:
		t.Fatal("OpenFile() emitted no exact-range report")
	}
}

func seedBuilders(t *testing.T) {
	t.Helper()
	for range maxTrackedOccurrences {
		active.currentRegistry().builders[new(strings.Builder)] = builderState{ranges: []Range{{Start: 0, End: 1}}}
	}
}

func seedStringReaders(t *testing.T) {
	t.Helper()
	for range maxTrackedOccurrences {
		active.currentRegistry().stringReaders[strings.NewReader("")] = readerState{}
	}
}

func seedBufioReaders(t *testing.T) {
	t.Helper()
	for range maxTrackedOccurrences {
		active.currentRegistry().bufioReaders[bufio.NewReader(strings.NewReader(""))] = readerState{}
	}
}
