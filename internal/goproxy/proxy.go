// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goproxy

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type config struct {
	toolexec string
}

type Option func(*config)

// WithToolexec forces a call to Run() to build with the -toolexec option when
// wrapping a build command
func WithToolexec(bin string, args ...string) Option {
	var buffer = strings.Builder{}
	if _, err := fmt.Fprintf(&buffer, "%q", bin); err != nil {
		// This is expected to never happen (short of running OOM, maybe?)
		panic(err)
	}

	for _, arg := range args {
		if buffer.Len() > 0 {
			buffer.WriteByte(' ')
		}
		// We are quoting all arguments to hopefully evade shell interpretation.
		if _, err := fmt.Fprintf(&buffer, "%q", arg); err != nil {
			// This is expected to never happen (short of running OOM, maybe?)
			panic(err)
		}
	}
	toolexec := buffer.String()
	buffer.Reset() // Dispose of the buffer's content so its memory can be recclaimed.
	return func(c *config) {
		c.toolexec = toolexec
	}
}

// Run takes a go directive (go build, go install, etc...) and applies
// changes specified through opts to the command before running it in a
// different process
func Run(args []string, opts ...Option) error {
	if len(args) == 0 {
		return fmt.Errorf("empty command line arguments")
	}

	var cfg config
	env := os.Environ()
	for _, opt := range opts {
		opt(&cfg)
	}

	goBin, err := goBin()
	if err != nil {
		return fmt.Errorf("locating 'go' binary: %w", err)
	}

	argV := make([]string, 1, len(args)+3)
	argV[0] = goBin
	argV = append(argV, args...)

	switch cmd := args[0]; cmd {
	// "go build" arguments are shared by build, clean, get, install, list, run, and test.
	case "build", "clean", "get", "install", "list", "run", "test":
		if cfg.toolexec != "" {
			oldLen := len(argV)
			// Add two slots to the argV array
			argV = append(argV, "", "")
			// Move all values from index 1 2 slots forward
			copy(argV[4:], argV[2:oldLen])
			// Fill in the two slots for toolexec.
			argV[2] = "-toolexec"
			argV[3] = cfg.toolexec
		}
	}

	return syscall.Exec(argV[0], argV, env)
}

var goBinPath string

// goBin returns the resolved path to the `go` command's binary. The result is cached to avoid
// looking it up multiple times. If the lookup fails, the error is returned and the result is not
// cached.
func goBin() (string, error) {
	if goBinPath == "" {
		goBin, err := exec.LookPath("go")
		if err != nil {
			return "", err
		}
		goBinPath = goBin
	}
	return goBinPath, nil
}
