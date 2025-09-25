// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goproxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/DataDog/orchestrion/internal/jobserver"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/traceutil"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
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
			_ = buffer.WriteByte(' ')
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

// BuildCmd returns a new exec.BuildCmd that will run the given goArgs, with the given opts applied.
func BuildCmd(ctx context.Context, goArgs []string, opts ...Option) (*exec.Cmd, error) {
	log := zerolog.Ctx(ctx)

	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	goArgs, err := processDashC(ctx, goArgs)
	if err != nil {
		return nil, err
	}

	goBin, err := goenv.GoBinPath()
	if err != nil {
		return nil, fmt.Errorf("locating 'go' binary: %w", err)
	}

	// Pre-allocate space for extra arguments...
	argv := append(
		append(
			make([]string, 0, len(goArgs)+3),
			goBin,
		),
		goArgs...,
	)

	var (
		server         *jobserver.Server
		serverStartErr error
		env            = os.Environ()
	)
	if len(argv) > 1 {
		switch cmd := argv[1]; cmd {
		// "go build" arguments are shared by build, clean, get, install, list, run, and test.
		case "build", "clean", "get", "install", "list", "run", "test":
			if cfg.toolexec != "" {
				log.Debug().Str("-toolexec", cfg.toolexec).Msg("Adding -toolexec argument")

				oldLen := len(argv)
				// Add two slots to the argV array
				argv = append(argv, "", "")
				// Move all values after the cmdIdx 2 slots forward
				copy(argv[4:], argv[2:oldLen])
				// Fill in the two slots for toolexec.
				argv[2] = "-toolexec"
				argv[3] = cfg.toolexec

				serverStarted := make(chan struct{})
				go func() {
					// We'll need a job server to support toolexec operations
					server, serverStartErr = jobserver.New(ctx, nil)
					if serverStartErr != nil {
						close(serverStarted)
						return
					}
					defer func() {
						server.Shutdown()
						log.Trace().Msg(server.CacheStats.String())
					}()
					close(serverStarted)

					<-ctx.Done()
				}()
				<-serverStarted
				env = append(env, fmt.Sprintf("%s=%s", client.EnvVarJobserverURL, server.ClientURL()))

				// Set the process' goflags, since we know them already...
				goflags.SetFlags(ctx, "", argv[1:])
			}
		}
	}
	if server != nil && serverStartErr != nil {
		return nil, serverStartErr
	}

	log.Trace().Strs("command", argv).Msg("exec")
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

// Run takes a go command ("build", "install", etc...) with its arguments, and
// applies changes specified through opts to the command before running it in a
// different process.
func Run(ctx context.Context, goArgs []string, opts ...Option) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd, err := BuildCmd(ctx, goArgs, opts...)
	if err != nil {
		return fmt.Errorf("building command: %w", err)
	}

	span, _ := tracer.StartSpanFromContext(ctx, "exec",
		tracer.ResourceName(cmd.String()),
	)
	defer span.Finish()
	tracer.Inject(span.Context(), traceutil.EnvVarCarrier{Env: &cmd.Env})

	if err := cmd.Run(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return cli.Exit(err, err.ExitCode())
		}
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}
