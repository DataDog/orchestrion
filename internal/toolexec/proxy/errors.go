package proxy

import "errors"

var (
	// ErrSkipCommand is returned by command processors to indicate that the
	// command should not be executed, and instead considered an idempotent
	// success.
	ErrSkipCommand = errors.New("skip command")
)
