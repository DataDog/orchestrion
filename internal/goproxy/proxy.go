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

	"github.com/datadog/orchestrion/internal/log"
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
func Run(goArgs []string, opts ...Option) error {
	var cfg config
	env := os.Environ()
	for _, opt := range opts {
		opt(&cfg)
	}

	goBin, err := GoBin()
	if err != nil {
		return fmt.Errorf("locating 'go' binary: %w", err)
	}

	// Pre-allocate space for extra arguments...
	argv := append(
		append(
			make([]string, 0, len(goArgs)+3),
			goBin,
		),
		goArgs...,
	)

	if len(argv) > 1 {
		// The command may not be at index 0, if the `-C` flag is used (it is REQUIRED to occur first
		// before anything else on the go command)
		cmdIdx := 1
		for {
			if cmdIdx+2 < len(argv) && argv[cmdIdx] == "-C" {
				cmdIdx += 2
			} else if cmdIdx+1 < len(argv) && strings.HasPrefix(argv[cmdIdx], "-C") {
				cmdIdx++
			} else {
				break
			}
		}

		switch cmd := argv[cmdIdx]; cmd {
		// "go build" arguments are shared by build, clean, get, install, list, run, and test.
		case "build", "clean", "get", "install", "list", "run", "test":
			if cfg.toolexec != "" {
				log.Debugf("Adding -toolexec=%q argument\n", cfg.toolexec)

				oldLen := len(argv)
				// Add two slots to the argV array
				argv = append(argv, "", "")
				// Move all values after the cmdIdx 2 slots forward
				copy(argv[cmdIdx+3:], argv[cmdIdx+1:oldLen])
				// Fill in the two slots for toolexec.
				argv[cmdIdx+1] = "-toolexec"
				argv[cmdIdx+2] = cfg.toolexec
			}
		}
	}

	log.Tracef("exec: %q\n", argv)
	return syscall.Exec(argv[0], argv, env)
}

var goBinPath string

// GoBin returns the resolved path to the `go` command's binary. The result is cached to avoid
// looking it up multiple times. If the lookup fails, the error is returned and the result is not
// cached.
func GoBin() (string, error) {
	if goBinPath == "" {
		goBin, err := exec.LookPath("go")
		if err != nil {
			return "", err
		}
		goBinPath = goBin
	}
	return goBinPath, nil
}
