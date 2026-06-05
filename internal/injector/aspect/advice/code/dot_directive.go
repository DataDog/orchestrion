// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"bufio"
	"regexp"
	"strings"
)

// DirectiveArgument represents arguments provided to directives (`//<directive> <args...>`), where
// arguments are parsed as space-separated key:value pairs.
type DirectiveArgument struct {
	Key   string // They key of the argument
	Value string // The value of the argument
}

var spaces = regexp.MustCompile(`\s+`)

// DirectiveArgs returns arguments provided to the named directive. A directive is a single-line
// comment with the directive immediately following the leading `//`, without any spacing in
// between; followed by optional arguments formatted as `key:value`, separated by spaces.
//
// Values might contain spaces, and in that case they need to be quoted either using single or double quotes as
// `key:"value with spaces"` or `key:'value with spaces'`.
func (d *dot) DirectiveArgs(directive string) []DirectiveArgument {
	prefix := "//" + directive

	for curr := d.context.Chain(); curr != nil; curr = curr.Parent() {
		for _, dec := range curr.Node().Decorations().Start {
			args, ok := parseDirectiveArgs(prefix, dec)
			if ok {
				return args
			}
		}
	}
	return nil
}

func parseDirectiveArgs(prefix string, comment string) ([]DirectiveArgument, bool) {
	if !strings.HasPrefix(comment, prefix) {
		return nil, false
	}
	parts := spaces.Split(comment, -1)
	if parts[0] != prefix {
		// This is not the directive we're looking for -- its name only starts the same.
		return nil, false
	}

	// Strip the prefix from the comment.
	argsStr := strings.TrimSpace(strings.TrimPrefix(comment, prefix))
	if argsStr == "" {
		return nil, true
	}

	scanner := bufio.NewScanner(strings.NewReader(argsStr))
	scanner.Split(splitArgs)

	var res []DirectiveArgument
	for scanner.Scan() {
		part := scanner.Text()
		if key, value, ok := strings.Cut(part, ":"); ok {
			if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
				(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
				value = value[1 : len(value)-1]
			}
			res = append(res, DirectiveArgument{Key: key, Value: value})
		} else {
			res = append(res, DirectiveArgument{Key: part, Value: ""})
		}
	}
	return res, true
}

func splitArgs(data []byte, atEOF bool) (advance int, token []byte, err error) {
	var (
		doubleQuote = false
		singleQuote = false
		start       = 0
	)
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '"':
			if !singleQuote {
				doubleQuote = !doubleQuote
			}
		case '\'':
			if !doubleQuote {
				singleQuote = !singleQuote
			}
		case ' ':
			if !doubleQuote && !singleQuote {
				if start < i {
					return i + 1, data[start:i], nil
				}
				start = i + 1
			}
		}
	}
	if atEOF && start < len(data) {
		return len(data), data[start:], nil
	}
	return 0, nil, nil
}
