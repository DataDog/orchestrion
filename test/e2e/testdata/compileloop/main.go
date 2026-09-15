// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Command compileloop builds a package that is instrumented with a dependency on
// a package that depends on it, which is the shape of build that deadlocks the
// job server's never-build-twice service.
package main

import (
	"fmt"

	"example.com/compileloop/victim"
)

func main() {
	fmt.Println(victim.Work())
}
