// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/goflags/quoted"
	"github.com/DataDog/orchestrion/internal/pin"
	"github.com/DataDog/orchestrion/internal/toolexec"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

// sanitizeGOFLAGS removes the `-toolexec` entry from this process's $GOFLAGS environment variable.
//
// The weaving process spawns nested `go` commands (e.g. `go env`, or the `go list -export` run by
// go/importer's default importer to locate standard library export data). If $GOFLAGS still
// contains `-toolexec`, these nested commands re-enter the weaving process for every package they
// build; this is at best needlessly expensive, and at worst breaks the build, as the nested
// `go list -export` runs with $GOROOT (which is not part of any module) as its working directory.
// Builds that genuinely need to be woven (e.g. resolving synthetic dependencies) opt back into
// `-toolexec` explicitly in their build flags, so they are unaffected.
func sanitizeGOFLAGS(log *zerolog.Logger) {
	goFlags := os.Getenv("GOFLAGS")
	if goFlags == "" {
		return
	}

	entries, err := quoted.Split(goFlags)
	if err != nil {
		log.Warn().Str("GOFLAGS", goFlags).Err(err).Msg("Failed to interpret quoted strings in $GOFLAGS; leaving it untouched")
		return
	}

	filtered := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry, "-toolexec") || strings.HasPrefix(entry, "--toolexec") {
			continue
		}
		filtered = append(filtered, entry)
	}
	if len(filtered) == len(entries) {
		return // No `-toolexec` entry, nothing to do.
	}

	joined, err := quoted.Join(filtered)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to re-quote $GOFLAGS; leaving it untouched")
		return
	}

	if err := os.Setenv("GOFLAGS", joined); err != nil {
		log.Warn().Err(err).Msg("Failed to update $GOFLAGS")
		return
	}
	log.Trace().Str("GOFLAGS", joined).Msg("Removed -toolexec from $GOFLAGS for this process")
}

func joinCommandCloseError(result error, closeErr error) error {
	if closeErr == nil {
		return result
	}
	if result == nil {
		return closeErr
	}
	if exitCoder, ok := result.(cli.ExitCoder); ok {
		return cli.Exit(errors.Join(result, closeErr), exitCoder.ExitCode())
	}
	return errors.Join(result, closeErr)
}

// relaxGarbageCollection makes the garbage collector run less often, unless the user configured it
// (GOGC or GOMEMLIMIT): toolexec processes are short-lived and allocation-heavy. Unlike setting GOGC,
// this leaves the go toolchain commands they run unaffected.
func relaxGarbageCollection() {
	if os.Getenv("GOGC") != "" || os.Getenv("GOMEMLIMIT") != "" {
		return
	}
	debug.SetGCPercent(400)
}

var Toolexec = &cli.Command{
	Name:            "toolexec",
	Usage:           "Standard `-toolexec` plugin for the Go toolchain",
	UsageText:       "orchestrion toolexec [tool] [tool args...]",
	Args:            true,
	SkipFlagParsing: true,
	Action: func(clictx *cli.Context) (resErr error) {
		relaxGarbageCollection()

		log := zerolog.Ctx(clictx.Context)
		sanitizeGOFLAGS(log)
		importPath := os.Getenv("TOOLEXEC_IMPORTPATH")

		span, ctx := tracer.StartSpanFromContext(clictx.Context, "toolexec",
			tracer.ResourceName(strings.Join(clictx.Args().Slice(), " ")),
		)
		defer func() { span.Finish(tracer.WithError(resErr)) }()

		proxyCmd, err := proxy.ParseCommand(ctx, importPath, clictx.Args().Slice())
		if err != nil || proxyCmd == nil {
			// An error occurred, or we have been instructed to skip this command.
			return err
		}
		defer func() { resErr = joinCommandCloseError(resErr, proxyCmd.Close(ctx, resErr)) }()

		if proxyCmd.Type() == proxy.CommandTypeOther {
			// Immediately run the command if it's of the Other type, as we do not do
			// any kind of processing on these...
			err := proxy.RunCommand(ctx, proxyCmd)
			var event *zerolog.Event
			if err != nil {
				event = log.Error().Err(err)
			} else {
				event = log.Trace()
			}
			event.Strs("command", proxyCmd.Args()).Msg("Toolexec fast-forward command")
			return err
		}

		// Ensure Orchestrion is properly pinned
		if err := pin.AutoPinOrchestrion(ctx, clictx.App.Writer, clictx.App.ErrWriter); err != nil {
			return cli.Exit(err, -1)
		}

		if proxyCmd.ShowVersion() {
			log.Trace().Strs("command", proxyCmd.Args()).Msg("Toolexec version command")
			fullVersion, err := toolexec.ComputeVersion(ctx, proxyCmd)
			if err != nil {
				return err
			}
			log.Trace().Str("version", fullVersion).Msg("Complete version output")
			_, err = fmt.Println(fullVersion)
			return err
		}

		log.Debug().Strs("command", proxyCmd.Args()).Msg("Toolexec original command")
		// NB: the raw `$TOOLEXEC_IMPORTPATH` value was passed to [proxy.ParseCommand], as it uniquely
		// identifies the compilation task; while the weaver is concerned with the package's identity.
		weaver := aspect.NewWeaver(importPath)

		if err := proxy.ProcessCommand(ctx, proxyCmd, weaver.OnCompile); errors.Is(err, proxy.ErrSkipCommand) {
			log.Trace().Msg("OnCompile processor requested command skipping...")
			return nil
		} else if err != nil {
			return err
		}
		if err := proxy.ProcessCommand(ctx, proxyCmd, weaver.OnCompileMain); errors.Is(err, proxy.ErrSkipCommand) {
			log.Trace().Msg("OnCompileMain processor requested command skipping...")
			return nil
		} else if err != nil {
			return err
		}
		if err := proxy.ProcessCommand(ctx, proxyCmd, weaver.OnLink); errors.Is(err, proxy.ErrSkipCommand) {
			log.Trace().Msg("OnLink processor requested command skipping...")
			return nil
		} else if err != nil {
			return err
		}

		log.Debug().Strs("command", proxyCmd.Args()).Msg("Toolexec final command")
		if err := proxy.RunCommand(ctx, proxyCmd); err != nil {
			// Logging as debug, as the error will likely surface back to the user anyway...
			log.Error().Strs("command", proxyCmd.Args()).Err(err).Msg("Proxied command failed")
			return err
		}
		return nil
	},
}
