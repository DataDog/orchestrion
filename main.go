// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"runtime/trace"
	"strconv"

	"github.com/datadog/orchestrion/internal/cmd"
	"github.com/datadog/orchestrion/internal/jobserver/client"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/version"
	"github.com/urfave/cli/v2"
)

const (
	envVarOrchestrionLogFile  = "ORCHESTRION_LOG_FILE"
	envVarOrchestrionLogLevel = "ORCHESTRION_LOG_LEVEL"
	envVarToolexecImportPath  = "TOOLEXEC_IMPORTPATH"
	envVarCPUProfiling        = "CPU_PROFILING"
	envVarHeapProfiling       = "HEAP_PROFILING"
	envVarExecutionTracing    = "EXECUTION_TRACING"
	envVarProfilePrefix       = "PROFILE_PREFIX"
)

var profilePrefix string

type ctxKeyCPUProfiling struct{}
type ctxKeyHeapProfiling struct{}
type ctxKeyExecutionTracing struct{}

func main() {
	// Setup the logger
	log.SetContext("ORCHESTRION", version.Tag)
	log.SetContext("PID", strconv.FormatInt(int64(os.Getpid()), 10))
	if val := os.Getenv(envVarToolexecImportPath); val != "" {
		log.SetContext(envVarToolexecImportPath, val)
	}
	defer log.Close()

	// Setup the CLI application
	app := cli.App{
		Name:        "orchestrion",
		Usage:       "Automatic compile-time instrumentation of Go code",
		Description: "Orchestrion automatically adds instrumentation to Go applications at compile-time by interfacing with the standard Go toolchain using the -toolexec mechanism to re-write source code before it is passed to the compiler.\n\nFor more information, visit https://datadoghq.dev/orchestrion",
		Copyright:   "2023-present Datadog, Inc.",
		HideVersion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Category: "Advanced",
				Name:     "job-server-url",
				EnvVars:  []string{client.ENV_VAR_JOBSERVER_URL},
				Usage:    "Set the job server URL",
				Hidden:   true, // Users don't normally need to use this.
				Action: func(_ *cli.Context, url string) error {
					// Forward the value to the environment variable, so that all child processes see it.
					return os.Setenv(client.ENV_VAR_JOBSERVER_URL, url)
				},
			},
			&cli.StringFlag{
				Category: "Logging",
				Name:     "log-level",
				EnvVars:  []string{envVarOrchestrionLogLevel},
				Usage:    "Set the log level (NONE, OFF, ERROR, WARN, INFO, DEBUG, TRACE)",
				Value:    "NONE",
				Action:   actionSetLogLevel,
			},
			&cli.StringFlag{
				Category: "Logging",
				Name:     "log-file",
				EnvVars:  []string{envVarOrchestrionLogFile},
				Usage:    "Send logging output to a file instead of STDERR. Unless --log-level is also specified, the default log level changed to WARN.",
				Action:   actionSetLogFile,
			},
			&cli.StringFlag{
				Category:    "Profiling",
				Name:        "profile-prefix",
				EnvVars:     []string{envVarProfilePrefix},
				Usage:       "Path prefix for profiling data",
				Value:       "",
				Destination: &profilePrefix,
				Action: func(ctx *cli.Context, s string) error {
					// Set the env var so child processes get it
					os.Setenv(envVarProfilePrefix, s)
					return nil
				},
				Hidden: true,
			},
			&cli.BoolFlag{
				Category: "Profiling",
				Name:     "cpu-profiling",
				EnvVars:  []string{envVarCPUProfiling},
				Usage:    "Enable the CPU profiler. Profiles are recorded to file \"orchestrion-cpu-${pid}.pprof\"",
				Action:   actionCPUProfiling,
				Hidden:   true,
			},
			&cli.BoolFlag{
				Category: "Profiling",
				Name:     "heap-profiling",
				EnvVars:  []string{envVarHeapProfiling},
				Usage:    "Enable the heap profiler. Profiles are recorded to file \"orchestrion-heap-${pid}.pprof\"",
				Action:   actionHeapProfiling,
				Hidden:   true,
			},
			&cli.BoolFlag{
				Category: "Profiling",
				Name:     "execution-tracing",
				EnvVars:  []string{envVarExecutionTracing},
				Usage:    "Enable the execution tracer. Traces are recorded to file \"orchestrion-${pid}.trace\"",
				Action:   actionExecutionTracing,
				Hidden:   true,
			},
		},
		Commands: []*cli.Command{
			cmd.Go,
			cmd.Pin,
			cmd.Toolexec,
			cmd.Version,
			cmd.Server,
		},
		After: func(ctx *cli.Context) error {
			// Stop profiling, execution tracing, if they were started
			if f := ctx.Context.Value(ctxKeyCPUProfiling{}); f != nil {
				pprof.StopCPUProfile()
				f.(*os.File).Close()
			}
			if f := ctx.Context.Value(ctxKeyExecutionTracing{}); f != nil {
				trace.Stop()
				f.(*os.File).Close()
			}
			if filename := ctx.Context.Value(ctxKeyHeapProfiling{}); filename != nil {
				filename := filename.(string)
				f, err := profileToFile(filename, func(w io.Writer) error {
					return pprof.Lookup("heap").WriteTo(w, 0)
				})
				if err != nil {
					return fmt.Errorf("writing heap profile: %w", err)
				}
				f.Close()
			}
			return nil
		},
	}
	// Run the CLI application
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

var logLevelSet bool

func actionSetLogLevel(_ *cli.Context, level string) error {
	if level, valid := log.LevelNamed(level); valid {
		logLevelSet = true
		log.SetLevel(level)
		return nil
	}
	return fmt.Errorf("invalid log level specified: %q", level)
}

func actionSetLogFile(_ *cli.Context, path string) error {
	if !filepath.IsAbs(path) {
		if wd, err := os.Getwd(); err == nil {
			path = filepath.Join(wd, path)
			os.Setenv(envVarOrchestrionLogFile, path)
		}
	}
	filename := os.Expand(path, func(name string) string {
		switch name {
		case "PID":
			log.SetContext("PID", "")
			return strconv.FormatInt(int64(os.Getpid()), 10)
		default:
			return fmt.Sprintf("$%s", name)
		}
	})
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	log.SetOutput(file)

	if !logLevelSet {
		log.SetLevel(log.LevelWarn)
	}

	return nil
}

func profilePath(nameFormat string) string {
	filename := fmt.Sprintf(nameFormat, os.Getpid())
	if profilePrefix != "" {
		filename = filepath.Join(profilePrefix, filename)
	}
	return filename
}

func profileToFile(filename string, collect func(io.Writer) error) (*os.File, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("creating file %s: %w", filename, err)
	}
	if err := collect(f); err != nil {
		f.Close()
		os.Remove(filename)
		return nil, fmt.Errorf("starting collection: %w", err)
	}
	return f, nil
}

func actionCPUProfiling(ctx *cli.Context, enabled bool) error {
	if !enabled {
		return nil
	}
	filename := profilePath("orchestrion-cpu-%d.pprof")
	f, err := profileToFile(filename, pprof.StartCPUProfile)
	if err != nil {
		return fmt.Errorf("starting CPU profiling: %w", err)
	}
	ctx.Context = context.WithValue(ctx.Context, ctxKeyCPUProfiling{}, f)
	os.Setenv(envVarCPUProfiling, "true")
	return nil
}

func actionHeapProfiling(ctx *cli.Context, enabled bool) error {
	if !enabled {
		return nil
	}
	filename := profilePath("orchestrion-heap-%d.pprof")
	ctx.Context = context.WithValue(ctx.Context, ctxKeyHeapProfiling{}, filename)
	os.Setenv(envVarHeapProfiling, "true")
	return nil
}

func actionExecutionTracing(ctx *cli.Context, enabled bool) error {
	if !enabled {
		return nil
	}
	filename := profilePath("orchestrion-%d.trace")
	f, err := profileToFile(filename, trace.Start)
	if err != nil {
		return fmt.Errorf("starting execution tracing: %w", err)
	}
	ctx.Context = context.WithValue(ctx.Context, ctxKeyExecutionTracing{}, f)
	os.Setenv(envVarExecutionTracing, "true")
	return nil
}
