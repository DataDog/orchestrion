// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"flag"
	"strings"
)

// parseToolFlags parses the provided Go tool invocation arguments using the
// given flag set, and returns the list of positional arguments.
//
// The Go toolchain regularly introduces new internal flags passed to tools such
// as `compile` and `link`; for example the `-exportfd` flag added to `compile`
// in Go 1.28 as part of the pipelined "early export data" work (see
// golang/go#15734). Because the flag sets used here are generated from the
// usage output of a specific Go release, they may not (yet) know about flags
// emitted by newer toolchains. Rather than failing the whole build, flags that
// are not defined in the flag set are ignored by the parser: they remain part
// of the original command arguments, and are therefore forwarded verbatim to
// the proxied tool when the command is re-executed (see [RunCommand]).
//
// Unknown flags are handled as follows:
//   - `-flag=value` (or `--flag=value`) is consumed as a single token;
//   - for the two-token `-flag value` form, the value token is consumed unless
//     it is absent, looks like another flag (starts with `-`), or looks like a
//     source or object file (.go, .a, .o) — in which case the flag is treated
//     as value-less and the next token is left among positional arguments.
//     Leaving a stray token among positionals is preferable to swallowing an
//     actual input file, which would corrupt [CompileCommand.Files] and
//     [LinkCommand.Inputs].
func parseToolFlags(flagSet *flag.FlagSet, args []string) ([]string, error) {
	unknown := make(map[int]struct{})
	for i := 0; i < len(args); {
		arg := args[i]
		if arg == "--" || len(arg) < 2 || arg[0] != '-' {
			// Positional argument (or terminator): everything from this point on
			// is positional, mirroring [flag.FlagSet.Parse] behavior.
			break
		}
		name := strings.TrimPrefix(arg[1:], "-")
		if name == "" {
			// A lone "-" is treated as a positional argument by the flag package.
			break
		}
		var hasValue bool
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name, hasValue = name[:eq], true
		}

		f := flagSet.Lookup(name)
		if f == nil {
			// Unknown flag: skip it, it is forwarded verbatim as part of the
			// original command arguments.
			unknown[i] = struct{}{}
			if !hasValue && unknownFlagTakesValue(args, i) {
				unknown[i+1] = struct{}{}
				i += 2
			} else {
				i++
			}
			continue
		}

		if hasValue || isBoolFlag(f.Value) {
			i++
			continue
		}
		// Known non-boolean flag in two-token form: skip the value token too,
		// mirroring [flag.FlagSet.Parse] behavior.
		i += 2
	}

	if len(unknown) == 0 {
		err := flagSet.Parse(args)
		return flagSet.Args(), err
	}

	filtered := make([]string, 0, len(args)-len(unknown))
	for i, arg := range args {
		if _, ok := unknown[i]; ok {
			continue
		}
		filtered = append(filtered, arg)
	}
	err := flagSet.Parse(filtered)
	return flagSet.Args(), err
}

// unknownFlagTakesValue reports whether the unknown flag at args[i] (given in
// two-token form) is assumed to be followed by a separate value token. See
// [parseToolFlags] for the exact semantics.
func unknownFlagTakesValue(args []string, i int) bool {
	if i+1 >= len(args) {
		return false
	}
	next := args[i+1]
	if strings.HasPrefix(next, "-") {
		// Looks like another flag, not a value.
		return false
	}
	for _, ext := range []string{".go", ".a", ".o"} {
		if strings.HasSuffix(next, ext) {
			// Looks like a source or object file: treat the flag as value-less
			// rather than risk swallowing an input file.
			return false
		}
	}
	return true
}

// isBoolFlag reports whether the given [flag.Value] is boolean-like, i.e. does
// not consume a separate value argument.
func isBoolFlag(v flag.Value) bool {
	bf, ok := v.(interface{ IsBoolFlag() bool })
	return ok && bf.IsBoolFlag()
}
