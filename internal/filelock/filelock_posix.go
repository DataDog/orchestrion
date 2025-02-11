// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build unix

package filelock

import (
	"os"
	"syscall"
)

// rlock places an advisory shared lock on the specified file.
func rlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH)
}

// lock places an advisory exclusive lock on the specified file.
func lock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlock removes any advisory locks from the specified file.
func unlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// beforeLockChange is called before the lock state is changed. It is a no-op on
// POSIX platforms, as [syscall.Flock] allows for a lock to be upgraded or
// downgraded freely. It returns `false` if the currently held lock is identical
// to the target state (idempotent), and always returns a `nil` error.
func (m *Mutex) beforeLockChange(to lockState) (cont bool, err error) {
	return m.locked != to, nil
}
