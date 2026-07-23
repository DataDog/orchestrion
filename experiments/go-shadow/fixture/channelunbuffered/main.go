// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func main() {
	values := make(chan string)
	done := make(chan struct{})
	go func() {
		values <- os.Getenv("TAINT_PATH")
		close(done)
	}()
	_, _ = os.Open(<-values)
	<-done
}
