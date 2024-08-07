// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build windows

package filelock

import (
	"os"

	"golang.org/x/sys/windows"
)

const (
	reserved = 0
	allBytes = ^uint32(0)
)

// rlock places an advisory shared lock on the specified file.
func rlock(f *os.File) error {
	return winLock(f, 0)
}

// lock places an advisory exclusive lock on the specified file.
func lock(f *os.File) error {
	return winLock(f, windows.LOCKFILE_EXCLUSIVE_LOCK)
}

func winLock(f *os.File, lockType uint32) error {
	var overlapped windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), lockType, reserved, allBytes, allBytes, &overlapped)
}

// unlock removes any advisory locks from the specified file.
func unlock(f *os.File) error {
	var overlapped windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), reserved, allBytes, allBytes, &overlapped)
}
