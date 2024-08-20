// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goproxy

import (
	"os"
	"strings"

	"github.com/datadog/orchestrion/internal/log"
)

// ProcessDashC gets the command line arguments passed to a "go" command (without "go" itself), and processes the "-C"
// flag present at the beginning of the slice (if present), changing directories as requested, then returns the slice
// without it.
//
// The "-C" flags is reuqired to be the very first argument provided to "go" commands.
func ProcessDashC(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}

	arg0 := args[0]
	if !strings.HasPrefix(arg0, "-C") {
		return args, nil
	}

	if arg0 == "-C" && len(args) > 1 {
		// ["-C", "directory", ...]
		log.Tracef("Found -C %q flag, changing directory\n", args[1])
		return args[2:], os.Chdir(args[1])
	}

	if !strings.HasPrefix(arg0, "-C=") {
		// Probably not the flag we're looking for... ignoring that...
		log.Tracef("Ignoring unknown flag with -C prefix: %q\n", arg0)
		return args, nil
	}

	// ["-C=directory", ...]
	log.Tracef("Found %q flag, changing directory\n", arg0)
	return args[1:], os.Chdir(arg0[3:])
}
