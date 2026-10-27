// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// This sample opts into the crashtracker integration. That integration is not
// enabled by default; the opt-in happens in this directory's
// `orchestrion.tool.go` file, which imports `dd-trace-go/v2/crashtracker`.
package main

import "log"

func main() {
	log.Println("hello, world")
}
