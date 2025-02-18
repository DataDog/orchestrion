// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package filelock

import (
	"context"
	"errors"
	"os"

	"github.com/rs/zerolog"
)

// Mutex is a file-based mutex intended to facilitate cross-process
// synchronization. Locks acquired by Mutex are advisory, so all participating
// processes must use advisory lock features in order to co-operate. Locks
// acquired by Mutex are not inherited by child processes and are automatically
// released when the process exits.
//
// It is not intended for in-process synchronization, and should not be shared
// between goroutines without being appropriately guarded by a sync.Mutex.
//
// Upgrading a read-lock to a write lock, or vice-versa, is not guaranteed to
// happen atomically (on Windows, it is guaranteed not to be atomic).
//
// The [io.ReadWriteSeeker], [io.ReaderAt], and [io.WriterAt] interfaces are
// implemented by [*Mutex] and are saef to be used once any lock as been
// acquired (as the result of [Mutex.RLock] or [Mutex.Lock]), and until the
// mutex has been released ([Mutex.Unlock]). Attempting to use these interfaces
// without a lock being held results in [fs.ErrClosed]. On UNIX platforms, it is
// generally OK to use different file descriptors to access the locked file, as
// the lock is held by the process, not the individual file descriptor. On
// Windows however, it is important to use the [Mutex] to perform IO operations,
// in particular when a write lock is used, as the lock is tied to the specific
// file descriptor.
type Mutex struct {
	file   *os.File
	path   string
	locked lockState
}

type lockState int

const (
	lockStateUnlocked lockState = iota
	lockStateRLocked
	lockStateWLocked
)

// MutexAt returns a new Mutex instance that will use the given path as the lock
// file.
func MutexAt(path string) *Mutex {
	return &Mutex{path: path}
}

// RLock attempts to lock the file for reading. It blocks until the lock is
// acquired, or an error happens. If the file is already locked for writing, it
// will downgrade the lock to a read-only lock.
func (m *Mutex) RLock(ctx context.Context) error {
	if m.file == nil {
		f, err := m.open()
		if err != nil {
			return err
		}
		m.file = f
	}

	log := zerolog.Ctx(ctx)
	if cont, err := m.beforeLockChange(ctx, lockStateRLocked); err != nil {
		log.Error().Err(err).Str("lock-file", m.path).Msg("Before lock change hook rejected action")
		return err
	} else if !cont {
		// Idempotent success!
		log.Trace().Str("lock-file", m.path).Msg("Before lock change hook detected idempotent operation")
		return nil
	}

	if err := rlock(m.file); err != nil {
		return err
	}

	log.Trace().Str("lock-file", m.path).Msg("Successfully acquired READ lock")
	m.locked = lockStateRLocked
	return nil
}

// Lock attempts to lock the file for reading & writing. It blocks until the
// lock is acquired, or an error happens. If the file is already locked for
// reading, it will upgrade the lock to a read-write lock.
func (m *Mutex) Lock(ctx context.Context) error {
	if m.file == nil {
		f, err := m.open()
		if err != nil {
			return err
		}
		m.file = f
	}

	log := zerolog.Ctx(ctx)
	if cont, err := m.beforeLockChange(ctx, lockStateWLocked); err != nil {
		log.Error().Err(err).Str("lock-file", m.path).Msg("Before lock change hook rejected action")
		return err
	} else if !cont {
		// Idempotent success!
		log.Trace().Str("lock-file", m.path).Msg("Before lock change hook detected idempotent operation")
		return nil
	}

	if err := lock(m.file); err != nil {
		return err
	}

	log.Trace().Str("lock-file", m.path).Msg("Successfully acquired WRITE lock")
	m.locked = lockStateWLocked
	return nil
}

// Unlock releases any lock acquired on the file.
func (m *Mutex) Unlock(ctx context.Context) error {
	if m.file == nil {
		return nil
	}

	if err := m.unlock(ctx); err != nil {
		return err
	}

	err := m.file.Close()
	if err == nil {
		m.file = nil
	}
	return err
}

// unlock releases the lock currently held on the file, but does not close it. This function is only
// safe to call if `m.file` is not `nil` (after [Mutex.open] was called, but before [Mutex.Unlock]
// is).
func (m *Mutex) unlock(ctx context.Context) error {
	log := zerolog.Ctx(ctx)

	if err := unlock(m.file); err != nil {
		log.Error().Err(err).Str("lock-file", m.path).Msg("Failed to unlock file")
		return err
	}

	log.Trace().Str("lock-file", m.path).Msg("Successfully unlocked file")
	m.locked = lockStateUnlocked
	return nil
}

func (m *Mutex) open() (*os.File, error) {
	if m.file != nil {
		return nil, errors.New("already opened")
	}

	return os.OpenFile(m.path, os.O_CREATE|os.O_RDWR, 0o644)
}
