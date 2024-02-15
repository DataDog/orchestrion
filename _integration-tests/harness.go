// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package integration

import (
	"os"
	"os/signal"
	"syscall"

	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace" // To have it pinned in the go.mod file
)

func OnSignal(f func()) {
	c := make(chan os.Signal, 0)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		f()
	}()
}
