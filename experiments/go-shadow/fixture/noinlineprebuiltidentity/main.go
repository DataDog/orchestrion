// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/experiments/go-shadow/fixture/noinlineprebuiltidentity/dependency"
)

func main() {
	path := os.Getenv("TAINT_PATH")
	if path == "" {
		path = "/tmp/iast-noinlineprebuiltidentity"
	}
	_, _ = os.Open(dependency.Identity(path))
}
