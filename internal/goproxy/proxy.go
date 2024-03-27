// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goproxy

import (
	"fmt"
	"log"
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
func WithToolexec(args ...string) Option {
	var buffer = strings.Builder{}
	for _, arg := range args {
		if buffer.Len() > 0 {
			buffer.WriteByte(' ')
		}
		// We are quoting all arguments to hopefully evade shell interpretation.
		_, err := fmt.Fprintf(&buffer, "%q", arg)
		if err != nil {
			// This is expected to never happen (short of running OOM, maybe?)
			panic(err)
		}
	}
	toolexec := buffer.String()
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

	cmd := args[0]
	switch cmd {
	// "go build" arguments are shared by build, clean, get, install, list, run, and test.
	case "build", "clean", "get", "install", "list", "run", "test":
		if cfg.toolexec != "" {
			newArgs := make([]string, len(args)+2)
			newArgs[0] = cmd
			newArgs[1] = "-toolexec"
			newArgs[2] = cfg.toolexec
			copy(newArgs[3:], args[1:])
			args = newArgs
		}
	default:
		break
	}

	args = append([]string{goBin}, args...)
	log.Printf("Executing '%v'", args)
	return syscall.Exec(goBin, args, env)
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
