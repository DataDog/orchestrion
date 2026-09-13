// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type hopper interface {
	hop(string) string
}

type first struct{}
type second struct{}

//go:noinline
func (first) hop(value string) string { return value }

//go:noinline
func (second) hop(value string) string { return value }

func main() {
	var target hopper
	if len(os.Args) > 1 {
		target = first{}
	} else {
		target = second{}
	}
	_, _ = os.Open(target.hop(os.Getenv("TAINT_PATH")))
}
