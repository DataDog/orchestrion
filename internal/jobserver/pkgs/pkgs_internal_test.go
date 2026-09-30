// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/DataDog/orchestrion/internal/goflags"
)

// TestMain sets the go command flags used by the job server, for the whole test binary (internal and
// external tests alike): they are process-wide, can only be set once (see [goflags.SetFlags]), and
// would otherwise be looked up from the `go test` command running this binary.
func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getting working directory: %v\n", err)
		os.Exit(1)
	}
	goflags.SetFlags(context.Background(), wd, []string{"test"})

	os.Exit(m.Run())
}
