// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"encoding/json"
	"os"
	"sync"
)

// Report describes tainted data observed at a sink.
type Report struct {
	Sink   string  `json:"sink"`
	Value  string  `json:"value"`
	Ranges []Range `json:"ranges"`
}

var reportState = struct {
	sync.RWMutex
	reporter func(Report)
}{reporter: writeJSONReport}

// SetReporter replaces the report consumer and returns a function that restores it.
func SetReporter(reporter func(Report)) func() {
	reportState.Lock()
	previous := reportState.reporter
	reportState.reporter = reporter
	reportState.Unlock()
	return func() {
		reportState.Lock()
		reportState.reporter = previous
		reportState.Unlock()
	}
}

// OpenFile is a drop-in os.Open sink that reports tainted path bytes.
func OpenFile(name string) (*os.File, error) {
	if ranges := RangesString(name); len(ranges) > 0 {
		emitReport(Report{Sink: "os.Open", Value: name, Ranges: ranges})
	}
	return os.Open(name)
}

func emitReport(report Report) {
	reportState.RLock()
	reporter := reportState.reporter
	reportState.RUnlock()
	reporter(report)
}

func writeJSONReport(report Report) {
	if os.Getenv("ORCHESTRION_TAINT_INCLUDE_VALUE") != "1" {
		report.Value = "[REDACTED]"
	}
	if err := json.NewEncoder(os.Stderr).Encode(report); err != nil {
		return
	}
}
