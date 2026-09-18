// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type case017Picker interface {
	Pick(string) string
}

type case017FirstReceiver string

type case017SecondReceiver string

//go:noinline
func (r case017FirstReceiver) Pick(string) string { return string(r) }

//go:noinline
func (r case017SecondReceiver) Pick(string) string { return string(r) }

func init() {
	register(Case{
		ID:   17,
		Name: "interface receiver stays clean",
		Run: func() {
			_ = os.Setenv("CASE_017_BRANCH", "second")
			_ = os.Setenv("CASE_017_INPUT", "secret")

			var target case017Picker
			if os.Getenv("CASE_017_BRANCH") == "second" {
				target = case017SecondReceiver("case017-second-clean")
			} else {
				target = case017FirstReceiver("case017-first-clean")
			}

			_, _ = os.Open(target.Pick(os.Getenv("CASE_017_INPUT")))
		},
		// Empirically confirmed via captured=[]: target.Pick is a genuine
		// dynamic interface dispatch (two implementing types, branch chosen
		// at runtime from CASE_017_BRANCH) whose result derives only from
		// the clean receiver literal; the tainted argument is discarded, and
		// no report fires. Want is omitted (no report expected at all).
	})
}
