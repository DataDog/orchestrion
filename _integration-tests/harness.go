// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package integration

import (
	"os"
	"os/signal"
	"syscall"
)

//go:generate go run ./utils/generator ./tests

// OnSignal is used in the config files from internal/injector/testdata.
func OnSignal(f func()) {
	c := make(chan os.Signal, 0)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		f()
	}()
}
