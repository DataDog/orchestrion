// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type Expect struct {
	Value  string
	Ranges []taint.Range
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
	for _, current := range cases {
		diagnostic := runCase(current)
		if diagnostic == "" {
			fmt.Printf("CASE\t%d\t%s\tPASS\n", current.ID, current.Name)
			continue
		}
		fmt.Printf("CASE\t%d\t%s\tFAIL\t%s\n", current.ID, current.Name, singleLine(diagnostic))
	}
}

func runCase(current Case) string {
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
		return fmt.Sprintf("panic: %v", panicValue)
	}
	return compareReports(reports, current.Want)
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
			if want.Ranges != nil && !slices.Equal(report.Ranges, want.Ranges) {
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

func singleLine(value string) string {
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(value)
}
