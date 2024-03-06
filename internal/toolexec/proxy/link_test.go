// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLink(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input []string
		stage string
		flags linkFlagSet
	}{
		{
			name:  "version print",
			input: []string{"/path/link", "-V=full"},
			stage: ".",
		},
		{
			name:  "link",
			input: []string{"/path/link", "-o", "/buildDir/b001/exe/a.out", "-importcfg", "/buildDir/b001/importcfg.link", "-buildmode=exe", "/buildDir/b001/_pkg_.a"},
			stage: "b001",
			flags: linkFlagSet{
				ImportCfg: "/buildDir/b001/importcfg.link",
				Output:    "/buildDir/b001/exe/a.out",
				BuildMode: "exe",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := parseLinkCommand(tc.input)
			require.NoError(t, err)
			require.Equal(t, CommandTypeLink, cmd.Type())
			require.Equal(t, tc.stage, cmd.Stage())
			c := cmd.(*LinkCommand)
			require.True(t, reflect.DeepEqual(tc.flags, c.Flags))
		})
	}
}
