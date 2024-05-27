// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build windows

package log

import (
	"os"

	"golang.org/x/sys/windows"
)

const allBytes = ^uint32(0)

// Flock sets an advisory lock for writing on the provided file.
func Flock(file *os.File) error {
	return windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, allBytes, allBytes, &windows.Overlapped{})
}

// FUnlock removes the advisory lock set by Flock.
func FUnlock(file *os.File) error {
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, allBytes, allBytes, &windows.Overlapped{})
}
