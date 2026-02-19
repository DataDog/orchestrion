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
	"path/filepath"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/goflags"
	injconfig "github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/jobserver"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/DataDog/orchestrion/internal/traceutil"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
	"golang.org/x/tools/go/packages"
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
				// Pre-compute GOMOD for toolexec children to avoid a subprocess
				// fork per package compilation. This is the single largest
				// per-invocation overhead (10-30ms each).
				if goMod, err := goenv.GOMOD("."); err == nil {
					env = append(env, fmt.Sprintf("%s=%s", goenv.EnvVarGoMod, goMod))
				}

				log.Debug().Str("-toolexec", cfg.toolexec).Msg("Adding -toolexec argument")

				oldLen := len(argv)
				// Add two slots to the argV array
				argv = append(argv, "", "")
				// Move all values after the cmdIdx 2 slots forward
				copy(argv[4:], argv[2:oldLen])
				// Fill in the two slots for toolexec.
				argv[2] = "-toolexec"
				argv[3] = cfg.toolexec

				log.Debug().Msg("Starting job server goroutine")
				serverStarted := make(chan struct{})
				go func() {
					// We'll need a job server to support toolexec operations
					log.Debug().Msg("Initializing job server")
					server, serverStartErr = jobserver.New(ctx, nil)
					if serverStartErr != nil {
						log.Error().Err(serverStartErr).Msg("Failed to start job server")
						close(serverStarted)
						return
					}
					log.Debug().Str("url", server.ClientURL()).Msg("Job server started successfully")
					defer func() {
						log.Debug().Msg("Shutting down job server")
						server.Shutdown()
						log.Trace().Msg(server.CacheStats.String())
						log.Debug().Msg("Job server shut down complete")
					}()
					close(serverStarted)

					<-ctx.Done()
					if ctxErr := ctx.Err(); ctxErr != nil {
						log.Debug().
							Err(ctxErr).
							Str("reason", ctxErr.Error()).
							Msg("Context cancelled, job server cleanup triggered")
					} else {
						log.Debug().Msg("Context done (normal shutdown), job server cleanup triggered")
					}
				}()
				<-serverStarted
				if serverStartErr == nil {
					log.Debug().Str("url", server.ClientURL()).Msg("Setting job server URL in environment")
					env = append(env, fmt.Sprintf("%s=%s", client.EnvVarJobserverURL, server.ClientURL()))

					// Pre-resolve config file paths so toolexec children can skip
					// the ~20 NATS round trips for package resolution.
					env = preResolveConfigFiles(ctx, log, server, env)
				}

				// Set the process' goflags, since we know them already...
				goflags.SetFlags(ctx, "", argv[1:])
			}
		}
	}
	if serverStartErr != nil {
		return nil, fmt.Errorf("job server failed to start: %w", serverStartErr)
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

	log := zerolog.Ctx(ctx)
	log.Debug().Str("command", cmd.String()).Msg("Starting command execution")
	if err := cmd.Run(); err != nil {
		log.Debug().Err(err).Str("command", cmd.String()).Msg("Command execution failed")
		if err, ok := err.(*exec.ExitError); ok {
			return cli.Exit(err, err.ExitCode())
		}
		return fmt.Errorf("exec: %w", err)
	}

	log.Debug().Str("command", cmd.String()).Msg("Command execution completed successfully")
	return nil
}

// preResolveConfigFiles uses the job server's in-process connection to resolve
// all config YAML file paths once, passing them to toolexec children via an
// environment variable. This collapses ~20 NATS round trips per toolexec
// invocation into zero.
func preResolveConfigFiles(ctx context.Context, log *zerolog.Logger, server *jobserver.Server, env []string) []string {
	goMod := os.Getenv(goenv.EnvVarGoMod)
	if goMod == "" {
		var err error
		goMod, err = goenv.GOMOD(".")
		if err != nil {
			log.Debug().Err(err).Msg("Cannot pre-resolve config files: GOMOD not available")
			return env
		}
	}
	goModDir := filepath.Dir(goMod)

	jsClient, err := server.Connect()
	if err != nil {
		log.Debug().Err(err).Msg("Cannot pre-resolve config files: failed to connect to job server")
		return env
	}
	defer jsClient.Close()

	pkgLoader := func(ctx context.Context, dir string, patterns ...string) ([]*packages.Package, error) {
		return client.Request(ctx, jsClient, pkgs.LoadRequest{Dir: dir, Patterns: patterns})
	}
	loader := injconfig.NewLoader(pkgLoader, goModDir, false)
	if _, err := loader.Load(ctx); err != nil {
		log.Debug().Err(err).Msg("Cannot pre-resolve config files: config loading failed")
		return env
	}

	files := loader.LoadedYMLFiles()
	if len(files) > 0 {
		val := strings.Join(files, string(os.PathListSeparator))
		env = append(env, fmt.Sprintf("%s=%s", injconfig.EnvVarConfigFiles, val))
		log.Debug().Int("count", len(files)).Msg("Pre-resolved config file paths for toolexec children")
	}
	return env
}
