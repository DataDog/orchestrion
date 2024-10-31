// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"runtime/trace"
	"slices"
	"strconv"
	"strings"

	"github.com/DataDog/orchestrion/internal/cmd"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/log"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/urfave/cli/v2"
)

const (
	envVarOrchestrionLogFile         = "ORCHESTRION_LOG_FILE"
	envVarOrchestrionLogLevel        = "ORCHESTRION_LOG_LEVEL"
	envVarOrchestrionProfilePath     = "ORCHESTRION_PROFILE_PATH"
	envVarOrchestrionEnabledProfiles = "ORCHESTRION_ENABLED_PROFILES"

	envVarToolexecImportPath = "TOOLEXEC_IMPORTPATH"
)

func main() {
	// Setup the logger
	log.SetContext("ORCHESTRION", version.Tag)
	log.SetContext("PID", strconv.FormatInt(int64(os.Getpid()), 10))
	if val := os.Getenv(envVarToolexecImportPath); val != "" {
		log.SetContext(envVarToolexecImportPath, val)
	}
	defer log.Close()

	var (
		cpuProfile     *os.File
		executionTrace *os.File
	)

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
				Name:     "C",
				Usage:    "Change to the specified directory before proceeding with the rest of the command.",
				Hidden:   true, // Users don't normally need to use this.
				Action: func(_ *cli.Context, dir string) error {
					return os.Chdir(dir)
				},
			},
			&cli.StringFlag{
				Category: "Advanced",
				Name:     "job-server-url",
				EnvVars:  []string{client.EnvVarJobserverURL},
				Usage:    "Set the job server URL",
				Hidden:   true, // Users don't normally need to use this.
				Action: func(_ *cli.Context, url string) error {
					// Forward the value to the environment variable, so that all child processes see it.
					return os.Setenv(client.EnvVarJobserverURL, url)
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
				EnvVars:  []string{envVarOrchestrionProfilePath},
				Usage:    "Path for profiling data. Defaults to the current working directory",
				Hidden:   true,
			},
			&cli.StringSliceFlag{
				Category: "Profiling",
				Name:     "profile",
				EnvVars:  []string{envVarOrchestrionEnabledProfiles},
				Usage:    "Enable the given profiler. Valid options are \"cpu\", \"heap\", and \"trace\". Can be specified multiple times.",
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
			if err := os.Setenv(envVarOrchestrionProfilePath, profilePath); err != nil {
				return cli.Exit(fmt.Errorf("setting environment %s: %w", envVarOrchestrionProfilePath, err), 1)
			}
			for _, p := range profiles {
				var err error
				switch p {
				case "heap":
					// Nothing to do; this is dealt with only in After
				case "cpu":
					cpuProfile, err = startCPUProfiling(profilePath)
				case "trace":
					executionTrace, err = startExecutionTracing(profilePath)
				default:
					return fmt.Errorf("unrecognized profile type %s", p)
				}
				if err != nil {
					return fmt.Errorf("enabling profile %s: %w", p, err)
				}
			}
			if err := os.Setenv(envVarOrchestrionEnabledProfiles, strings.Join(profiles, ",")); err != nil {
				return cli.Exit(fmt.Errorf("setting environment %s: %w", envVarOrchestrionEnabledProfiles, err), 1)
			}
			return nil
		},
		After: func(ctx *cli.Context) error {
			// Stop profiling, execution tracing, if they were started
			if cpuProfile != nil {
				pprof.StopCPUProfile()
				if err := cpuProfile.Close(); err != nil {
					log.Warnf("Failed to close CPU profile: %v\n", err)
				}
			}
			if executionTrace != nil {
				trace.Stop()
				if err := executionTrace.Close(); err != nil {
					log.Warnf("Failed to close execution trace: %v\n", err)
				}
			}
			if slices.Contains(ctx.StringSlice("profile"), "heap") {
				filename := profilePath(ctx.String("profile-path"), "orchestrion-heap-%d.pprof")
				f, err := profileToFile(filename, func(w io.Writer) error {
					return pprof.Lookup("heap").WriteTo(w, 0)
				})
				if err != nil {
					return fmt.Errorf("writing heap profile: %w", err)
				}
				if err := f.Close(); err != nil {
					log.Warnf("Failed to close heap profile: %v\n", err)
				}
			}
			return nil
		},
	}
	// Run the CLI application
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

var logLevelSet bool

func actionSetLogLevel(_ *cli.Context, level string) error {
	if err := os.Setenv(envVarOrchestrionLogLevel, level); err != nil {
		return cli.Exit(fmt.Errorf("setting environment %s: %w", envVarOrchestrionLogLevel, err), 1)
	}
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
		}
	}
	if err := os.Setenv(envVarOrchestrionLogFile, path); err != nil {
		return cli.Exit(fmt.Errorf("setting environment %s: %w", envVarOrchestrionLogFile, err), 1)
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

func profilePath(path string, nameFormat string) string {
	return filepath.Join(path, fmt.Sprintf(nameFormat, os.Getpid()))
}

func profileToFile(filename string, collect func(io.Writer) error) (*os.File, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("creating file %s: %w", filename, err)
	}
	if err := collect(f); err != nil {
		err = errors.Join(err, f.Close())
		err = errors.Join(err, os.Remove(filename))
		return nil, fmt.Errorf("starting collection: %w", err)
	}
	return f, nil
}

func startCPUProfiling(prefix string) (*os.File, error) {
	filename := profilePath(prefix, "orchestrion-cpu-%d.pprof")
	f, err := profileToFile(filename, pprof.StartCPUProfile)
	if err != nil {
		return nil, fmt.Errorf("starting CPU profiling: %w", err)
	}
	return f, nil
}

func startExecutionTracing(prefix string) (*os.File, error) {
	filename := profilePath(prefix, "orchestrion-%d.trace")
	f, err := profileToFile(filename, trace.Start)
	if err != nil {
		return nil, fmt.Errorf("starting execution tracing: %w", err)
	}
	return f, nil
}
