// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type picker interface {
	pick(string) string
}

type firstReceiver string
type secondReceiver string

//go:noinline
func (receiver firstReceiver) pick(string) string { return string(receiver) }

//go:noinline
func (receiver secondReceiver) pick(string) string { return string(receiver) }

func main() {
	var target picker
	if len(os.Args) > 1 {
		target = firstReceiver("clean")
	} else {
		target = secondReceiver("clean")
	}
	_, _ = os.Open(target.pick(os.Getenv("TAINT_PATH")))
}
