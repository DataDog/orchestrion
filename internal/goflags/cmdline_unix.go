// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build !windows

package goflags

import (
	"context"

	"github.com/shirou/gopsutil/v4/process"
)

// CmdlineSlice returns the command line arguments of the process as a string
// slice. This is akin to calling [process.Process.CmdlineSliceWithContext],
// expect it properly parses Windows command line arguments.
func CmdlineSlice(ctx context.Context, proc *process.Process) ([]string, error) {
	return proc.CmdlineSliceWithContext(ctx)
}
