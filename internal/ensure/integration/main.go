// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"
	"log"

	"github.com/datadog/orchestrion/internal/ensure"
)

// main is the entry point of a command that is used by the `ensure` integration
// test to verify the `ensure.RequiredVersion()` function behaves correctly in
// real conditions.
func main() {
	if err := ensure.RequiredVersion(); err != nil {
		log.Fatalln(err)
	}

	_, _ = fmt.Println("This command has not respawned!")
}
