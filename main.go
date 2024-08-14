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
	"strings"

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
	envVarProfilePath         = "ORCHESTRION_PROFILE_PATH"
	envVarEnabledProfiles     = "ORCHESTRION_ENABLED_PROFILES"
)

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
				Category: "Profiling",
				Name:     "profile-path",
				EnvVars:  []string{envVarProfilePath},
				Usage:    "Path for profiling data. Defaults to the current working directory",
				Hidden:   true,
			},
			&cli.StringSliceFlag{
				Category: "Profiling",
				Name:     "profile",
				EnvVars:  []string{envVarEnabledProfiles},
				Usage:    "Enable the given profiler. Valid options are \"cpu\", \"heap\", and \"trace\"",
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
		Before: func(ctx *cli.Context) error {
			profiles := ctx.StringSlice("profile")
			if len(profiles) == 0 {
				return nil
			}

			profilePath, err := filepath.Abs(ctx.String("profile-path"))
			if err != nil {
				return err
			}
			if err := os.MkdirAll(profilePath, 0775); err != nil && !os.IsExist(err) {
				return err
			}
			os.Setenv(envVarProfilePath, profilePath)
			for _, p := range profiles {
				var err error
				switch p {
				case "heap":
					err = enableHeapProfiling(ctx, profilePath)
				case "cpu":
					err = enableCPUProfiling(ctx, profilePath)
				case "trace":
					err = enableExecutionTracing(ctx, profilePath)
				default:
					return fmt.Errorf("unrecognized profile type %s", p)
				}
				if err != nil {
					return fmt.Errorf("enabling profile %s: %w", p, err)
				}
			}
			os.Setenv(envVarEnabledProfiles, strings.Join(profiles, ","))
			return nil
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

func profilePath(path, nameFormat string) string {
	return filepath.Join(path, fmt.Sprintf(nameFormat, os.Getpid()))
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

func enableCPUProfiling(ctx *cli.Context, prefix string) error {
	filename := profilePath(prefix, "orchestrion-cpu-%d.pprof")
	f, err := profileToFile(filename, pprof.StartCPUProfile)
	if err != nil {
		return fmt.Errorf("starting CPU profiling: %w", err)
	}
	ctx.Context = context.WithValue(ctx.Context, ctxKeyCPUProfiling{}, f)
	return nil
}

func enableHeapProfiling(ctx *cli.Context, prefix string) error {
	filename := profilePath(prefix, "orchestrion-heap-%d.pprof")
	ctx.Context = context.WithValue(ctx.Context, ctxKeyHeapProfiling{}, filename)
	return nil
}

func enableExecutionTracing(ctx *cli.Context, prefix string) error {
	filename := profilePath(prefix, "orchestrion-%d.trace")
	f, err := profileToFile(filename, trace.Start)
	if err != nil {
		return fmt.Errorf("starting execution tracing: %w", err)
	}
	ctx.Context = context.WithValue(ctx.Context, ctxKeyExecutionTracing{}, f)
	return nil
}
