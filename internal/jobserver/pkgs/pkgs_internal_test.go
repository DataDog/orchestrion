// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"os"
	"testing"

	"github.com/DataDog/orchestrion/internal/goflags"
)

// TestMain sets the go command flags used by the job server, for the whole test binary (internal and
// external tests alike): they are process-wide, can only be set once (see [goflags.SetFlags]), and
// would otherwise be looked up from the `go test` command running this binary.
func TestMain(m *testing.M) {
	goflags.SetFlags(context.Background(), "", []string{"test"})
	os.Exit(m.Run())
}
