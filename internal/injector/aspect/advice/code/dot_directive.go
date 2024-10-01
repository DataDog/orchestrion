// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
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
func (d *dot) DirectiveArgs(directive string) (args []DirectiveArgument) {
	prefix := fmt.Sprintf(`//%s`, directive)

	for curr := context.AspectContext(d.context); curr != nil; curr = curr.Parent() {
		for _, dec := range curr.Node().Decorations().Start {
			if !strings.HasPrefix(dec, prefix) {
				continue
			}
			parts := spaces.Split(dec, -1)
			if parts[0] != prefix {
				// This is not the directive we're looking for -- its name only starts the same.
				continue
			}
			for _, part := range parts[1:] {
				key, value, _ := strings.Cut(part, ":")
				args = append(args, DirectiveArgument{Key: key, Value: value})
			}
			return
		}
	}
	return
}
