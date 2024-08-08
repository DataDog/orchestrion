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

	"github.com/datadog/orchestrion/internal/goenv"
	"github.com/datadog/orchestrion/internal/goflags"
	"github.com/datadog/orchestrion/internal/jobserver"
	"github.com/datadog/orchestrion/internal/jobserver/client"
	"github.com/datadog/orchestrion/internal/log"
)

type config struct {
	toolexec string
}

type Option func(*config)

// WithToolexec forces a call to Run() to build with the -toolexec option when
// wrapping a build command
func WithToolexec(bin string, args ...string) Option {
	var buffer strings.Builder
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

// Run takes a go sub-command (build, install, etc...) with its arguments, and
// applies changes specified through opts to the command before running it in a
// different process.
func Run(goArgs []string, opts ...Option) error {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	if len(goArgs) > 0 && strings.HasPrefix(goArgs[0], "-C") {
		// If the first argument is "-C", we'll change working directories of the
		// child process before spawning it. Normally, the `go` command does this, but
		// we'd need to detect the "-C" argument and skip it to identify the go
		// command name anyway, so...
		var dir string
		if goArgs[0] == "-C" && len(goArgs) > 1 {
			// ["-C", dir]
			dir = goArgs[1]
			goArgs = goArgs[2:]
		} else if strings.HasPrefix(goArgs[0], "-C=") {
			// ["-C=dir"]
			dir = goArgs[0][3:]
			goArgs = goArgs[1:]
		}

		if dir != "" {
			if err := os.Chdir(dir); err != nil {
				return fmt.Errorf("failed to change directory to %q: %w", dir, err)
			}
			log.Tracef("Changed directory to %q\n", dir)
		}
	}

	goBin, err := goenv.GoBinPath()
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

	env := os.Environ()
	if len(argv) > 1 {
		switch cmd := argv[1]; cmd {
		// "go build" arguments are shared by build, clean, get, install, list, run, and test.
		case "build", "clean", "get", "install", "list", "run", "test":
			if cfg.toolexec != "" {
				log.Debugf("Adding -toolexec=%q argument\n", cfg.toolexec)

				oldLen := len(argv)
				// Add two slots to the argV array
				argv = append(argv, "", "")
				// Move all values after the cmdIdx 2 slots forward
				copy(argv[4:], argv[2:oldLen])
				// Fill in the two slots for toolexec.
				argv[2] = "-toolexec"
				argv[3] = cfg.toolexec

				// We'll need a job server to support toolexec operations
				server, err := jobserver.New(&jobserver.Options{ServerName: fmt.Sprintf("orchestrion[%d]", os.Getpid())})
				if err != nil {
					return err
				}
				defer func() {
					server.Shutdown()
					log.Tracef("[JOBSERVER]: %s\n", server.CacheStats.String())
				}()
				env = append(env, fmt.Sprintf("%s=%s", client.ENV_VAR_JOBSERVER_URL, server.ClientURL()))
				// Set the `goflags` since we already know what they are...
				goflags.SetFlags(argv[2:])
			}
		}
	}

	log.Tracef("exec: %q\n", argv)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			os.Exit(err.ExitCode())
		}
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}
