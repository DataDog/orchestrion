// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import "testing"

func Test_StartRequestPreservesEscapedRanges_when_GoroutineReceivesPriorRequestValue(t *testing.T) {
	// Given
	active = newRegistryLifecycle()
	t.Cleanup(func() { active = newRegistryLifecycle() })
	reports := make(chan Report, 2)
	restore := SetReporter(func(report Report) { reports <- report })
	t.Cleanup(restore)
	value := SourceString("/missing/escaped")
	values := make(chan string, 1)
	proceed := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		path := <-values
		<-proceed
		file, _ := OpenFile(path)
		if file != nil {
			_ = file.Close()
		}
		close(finished)
	}()
	values <- value

	// When
	StartRequest()
	clean := isolatedString(value)
	valueStart, _ := stringAddress(value)
	cleanStart, _ := stringAddress(clean)
	if cleanStart == valueStart {
		t.Fatal("clean request-B value aliases request-A storage")
	}
	file, _ := OpenFile(clean)
	if file != nil {
		_ = file.Close()
	}
	select {
	case report := <-reports:
		t.Fatalf("OpenFile() emitted clean request-B report %#v", report)
	default:
	}
	close(proceed)
	<-finished

	// Then
	select {
	case report := <-reports:
		if report.Sink != "os.Open" || report.Value != value || report.State != "" {
			t.Fatalf("report = %#v, want exact report for %q", report, value)
		}
		requireRanges(t, report.Ranges, Range{Start: 0, End: len(value)})
	default:
		t.Fatal("OpenFile() emitted no escaped-value report")
	}
}

func Test_StartRequestReportsUnknown_when_EscapedValueIsEvicted(t *testing.T) {
	// Given
	active = newRegistryLifecycle()
	t.Cleanup(func() { active = newRegistryLifecycle() })
	reports := make(chan Report, 2)
	restore := SetReporter(func(report Report) { reports <- report })
	t.Cleanup(restore)
	value := SourceString("/missing/evicted")
	values := make(chan string, 1)
	proceed := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		path := <-values
		<-proceed
		file, _ := OpenFile(path)
		if file != nil {
			_ = file.Close()
		}
		close(finished)
	}()
	values <- value

	// When
	StartRequest()
	clean := isolatedString(value)
	file, _ := OpenFile(clean)
	if file != nil {
		_ = file.Close()
	}
	select {
	case report := <-reports:
		t.Fatalf("OpenFile() emitted clean retained-generation report %#v", report)
	default:
	}
	StartRequest()
	if len(active.retired) != 1 {
		t.Fatalf("retired generations = %d, want 1", len(active.retired))
	}
	close(proceed)
	<-finished

	// Then
	select {
	case report := <-reports:
		if report.Sink != "os.Open" || report.Value != value || report.State != Unknown || len(report.Ranges) != 0 {
			t.Fatalf("report = %#v, want unknown report without ranges", report)
		}
	default:
		t.Fatal("OpenFile() emitted no evicted-value report")
	}
}

// Test_StartRequestOverTaintsCleanSinks_when_GenerationHistoryIsDiscarded pins the
// COST of the request-scoped reset, not a capability.
//
// historyDiscarded is a process-wide latch: StartRequest sets it the first time it
// evicts a generation and nothing ever clears it. From that point
// stringRangesAndSaturation reports saturation for every value, so reportOpenPath
// takes its State=Unknown branch for any path it cannot resolve to concrete ranges
// - including literals that were never derived from a source.
//
// The direction of the approximation is what makes the reset sound: an escaped value
// whose generation is gone degrades to Unknown, never to silently clean, so there is
// no false negative. The price is a total false-positive rate on clean sinks for the
// remaining process lifetime, which is why case 108 is recorded Partial and not Win.
//
// This test fails if that trade is ever inverted into a false negative.
func Test_StartRequestOverTaintsCleanSinks_when_GenerationHistoryIsDiscarded(t *testing.T) {
	// Given
	active = newRegistryLifecycle()
	t.Cleanup(func() { active = newRegistryLifecycle() })
	var reports []Report
	restore := SetReporter(func(report Report) { reports = append(reports, report) })
	t.Cleanup(restore)

	openClean := func(path string) {
		file, _ := OpenFile(path)
		if file != nil {
			_ = file.Close()
		}
	}

	// A clean literal is silent while no generation has been discarded.
	openClean("/missing/clean-baseline")
	if len(reports) != 0 {
		t.Fatalf("baseline clean open reports = %#v, want none", reports)
	}

	// When: two generations start, so the first is evicted and the latch is set.
	StartRequest()
	StartRequest()
	if !active.historyDiscarded {
		t.Fatal("historyDiscarded = false, want true after an eviction")
	}
	if len(active.retired) != maxRetiredGenerations {
		t.Fatalf("retired generations = %d, want %d", len(active.retired), maxRetiredGenerations)
	}

	reports = nil
	for _, path := range []string{"/missing/clean-a", "/missing/clean-b", "/missing/clean-c"} {
		openClean(path)
	}

	// Then: every clean open now over-taints to Unknown, with no ranges and no value loss.
	if len(reports) != 3 {
		t.Fatalf("clean-open reports = %d (%#v), want 3 conservative Unknown reports", len(reports), reports)
	}
	for index, report := range reports {
		if report.State != Unknown {
			t.Errorf("report[%d].State = %q, want %q (a clean sink must never degrade to a precise or absent report here)", index, report.State, Unknown)
		}
		if len(report.Ranges) != 0 {
			t.Errorf("report[%d].Ranges = %#v, want none: saturation must not invent byte ranges", index, report.Ranges)
		}
	}
}
