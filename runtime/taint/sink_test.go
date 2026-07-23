// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "testing"

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
