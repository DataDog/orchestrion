// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goproxy

import (
	"context"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// processDashC gets the command line arguments passed to a "go" command (without "go" itself), and processes the "-C"
// flag present at the beginning of the slice (if present), changing directories as requested, then returns the slice
// without it.
//
// The "-C" flags is required to be the very first argument provided to "go" commands.
func processDashC(ctx context.Context, args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}

	log := zerolog.Ctx(ctx)

	arg0 := args[0]
	if !strings.HasPrefix(arg0, "-C") {
		return args, nil
	}

	if arg0 == "-C" && len(args) > 1 {
		// ["-C", "directory", ...]
		log.Trace().Str("-C", args[1]).Msg("Found '-C <path>' flag, changing directory")
		return args[2:], os.Chdir(args[1])
	}

	if !strings.HasPrefix(arg0, "-C=") {
		// Probably not the flag we're looking for... ignoring that...
		log.Trace().Str("flag", arg0).Msg("Ignoring unknown flag with -C prefix")
		return args, nil
	}

	// ["-C=directory", ...]
	log.Trace().Str("-C", arg0[3:]).Msg("Found '-C=<path>' flag, changing directory")
	return args[1:], os.Chdir(arg0[3:])
}
