// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build windows

package goflags

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
)

// CmdlineSlice returns the command line arguments of the process as a string
// slice. This is akin to calling [process.Process.CmdlineSliceWithContext],
// expect it properly parses Windows command line arguments.
func CmdlineSlice(ctx context.Context, proc *process.Process) ([]string, error) {
	cmdline, err := proc.CmdlineWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return commandLineToArgv(cmdline)
}

func commandLineToArgv(cmdline string) ([]string, error) {
	cmdlineptr, err := windows.UTF16PtrFromString(cmdline)
	if err != nil {
		return nil, fmt.Errorf("encoding cmdline as UTF-16: %w", err)
	}

	var argc int32
	argvptr, err := windows.CommandLineToArgv(cmdlineptr, &argc)
	if err != nil {
		return nil, fmt.Errorf("parsing windows command line arguments: %w", err)
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(argvptr))))

	argv := make([]string, argc)
	for i, v := range (*argvptr)[:argc] {
		argv[i] = windows.UTF16ToString((*v)[:])
	}
	return argv, nil
}
