package proxy

import "errors"

var (
	// SkipCommand is returned by command processors to indicate that the command
	// should not be executed, and instead considered an idempotent success.
	SkipCommand = errors.New("skip command")
)
