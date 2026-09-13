// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"runtime"
)

func sink(path string) {
	_, _ = os.Open(path)
}

func bufferedSend() {
	values := make(chan string, 1)
	blocked := make(chan string)
	select {
	case values <- os.Getenv("TAINT_PATH"):
	case blocked <- "":
	}
	sink(<-values)
}

func blockingReceive() {
	values := make(chan string)
	blocked := make(chan string)
	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(ready)
		var path string
		select {
		case path = <-values:
		case path = <-blocked:
		}
		sink(path)
		close(done)
	}()
	<-ready
	runtime.Gosched()
	values <- os.Getenv("TAINT_PATH")
	<-done
}

func blockingSend() {
	values := make(chan string)
	blocked := make(chan string)
	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		path := os.Getenv("TAINT_PATH")
		close(ready)
		select {
		case values <- path:
		case blocked <- path:
		}
		close(done)
	}()
	<-ready
	runtime.Gosched()
	sink(<-values)
	<-done
}

func immediateClosedReceive() {
	values := make(chan string)
	blocked := make(chan string)
	close(values)
	path := os.Getenv("TAINT_PATH")
	select {
	case path = <-values:
	case path = <-blocked:
	}
	sink(path)
}

func blockingClosedReceive() {
	values := make(chan string)
	blocked := make(chan string)
	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		path := os.Getenv("TAINT_PATH")
		close(ready)
		select {
		case path = <-values:
		case path = <-blocked:
		}
		sink(path)
		close(done)
	}()
	<-ready
	runtime.Gosched()
	close(values)
	<-done
}

func main() {
	bufferedSend()
	blockingReceive()
	blockingSend()
	immediateClosedReceive()
	blockingClosedReceive()
}
