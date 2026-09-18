// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type Expect struct {
	Value             string
	Ranges            []taint.Range
	DistinctSourceIDs int
}

type Case struct {
	ID   int
	Name string
	Run  func()
	Want []Expect
}

var cases []Case

func register(current Case) {
	cases = append(cases, current)
}

func main() {
	sort.Slice(cases, func(left, right int) bool {
		if cases[left].ID == cases[right].ID {
			return cases[left].Name < cases[right].Name
		}
		return cases[left].ID < cases[right].ID
	})

	// The driver parses stdout line by line and rejects anything that is not a CASE
	// record, so observations go to a file instead of the console.
	observations := newObservationSink(os.Getenv("IAST_E2E_OBSERVED"))
	defer observations.close()

	for _, current := range cases {
		reports, diagnostic := runCase(current)
		observations.record(current, reports, diagnostic)
		if diagnostic == "" {
			fmt.Printf("CASE\t%d\t%s\tPASS\n", current.ID, current.Name)
			continue
		}
		fmt.Printf("CASE\t%d\t%s\tFAIL\t%s\n", current.ID, current.Name, singleLine(diagnostic))
	}
}

func runCase(current Case) ([]taint.Report, string) {
	reports := make([]taint.Report, 0, len(current.Want))
	restore := taint.SetReporter(func(report taint.Report) {
		reports = append(reports, report)
	})
	var panicValue any
	func() {
		defer restore()
		defer func() {
			panicValue = recover()
		}()
		current.Run()
	}()
	if panicValue != nil {
		return reports, fmt.Sprintf("panic: %v", panicValue)
	}
	return reports, compareReports(reports, current.Want)
}

// observation is what a case actually produced, as opposed to what it asserted. A passing
// case proves observation and expectation agree, but only the observation can be quoted as
// measured evidence, so IAST_E2E_OBSERVED makes it recoverable without weakening any
// assertion or changing the console protocol.
type observation struct {
	ID         int            `json:"id"`
	Name       string         `json:"name"`
	Passed     bool           `json:"passed"`
	Diagnostic string         `json:"diagnostic,omitempty"`
	Reports    []taint.Report `json:"reports"`
}

type observationSink struct {
	file    *os.File
	encoder *json.Encoder
}

func newObservationSink(path string) *observationSink {
	if path == "" {
		return &observationSink{}
	}
	file, err := os.Create(path)
	if err != nil {
		// Observation is a diagnostic aid; never fail a test run over it.
		fmt.Fprintf(os.Stderr, "e2e: cannot write observations to %s: %v\n", path, err)
		return &observationSink{}
	}
	return &observationSink{file: file, encoder: json.NewEncoder(file)}
}

func (sink *observationSink) record(current Case, reports []taint.Report, diagnostic string) {
	if sink.encoder == nil {
		return
	}
	_ = sink.encoder.Encode(observation{
		ID:         current.ID,
		Name:       current.Name,
		Passed:     diagnostic == "",
		Diagnostic: singleLine(diagnostic),
		Reports:    reports,
	})
}

func (sink *observationSink) close() {
	if sink.file != nil {
		_ = sink.file.Close()
	}
}

func compareReports(reports []taint.Report, wants []Expect) string {
	matched := make([]bool, len(reports))
	for _, want := range wants {
		found := false
		for index, report := range reports {
			if matched[index] || report.Sink != "os.Open" || report.Value != want.Value {
				continue
			}
			if want.Ranges == nil && len(report.Ranges) == 0 {
				continue
			}
			if want.Ranges != nil && !sameRangeCoordinates(report.Ranges, want.Ranges) {
				continue
			}
			if want.DistinctSourceIDs != 0 && distinctSourceIDCount(report.Ranges) != want.DistinctSourceIDs {
				continue
			}
			matched[index] = true
			found = true
			break
		}
		if !found {
			return fmt.Sprintf("missing report value=%q ranges=%v; captured=%v", want.Value, want.Ranges, reports)
		}
	}
	for index, report := range reports {
		if !matched[index] {
			return fmt.Sprintf("unexpected report sink=%q value=%q ranges=%v", report.Sink, report.Value, report.Ranges)
		}
	}
	return ""
}

func sameRangeCoordinates(actual, expected []taint.Range) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index].Start != expected[index].Start || actual[index].End != expected[index].End {
			return false
		}
	}
	return true
}

func distinctSourceIDCount(ranges []taint.Range) int {
	ids := make(map[uint64]struct{}, len(ranges))
	for _, current := range ranges {
		if current.SourceID != 0 {
			ids[current.SourceID] = struct{}{}
		}
	}
	return len(ids)
}

func singleLine(value string) string {
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(value)
}
