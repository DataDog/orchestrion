// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"strings"
)

func init() {
	register(Case{
		ID:   0,
		Name: "strings builder clean write",
		Run: func() {
			var builder strings.Builder
			_, _ = builder.WriteString("builder-")
			_, _ = builder.Write([]byte("clean"))
			_, _ = os.Open(builder.String())
		},
	})
}
