// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build tools

package tools

import (
	// Tool dependencies
	_ "github.com/google/go-licenses/v2"
	_ "golang.org/x/perf/cmd/benchstat" // Used in GitHub Workflow validate.yml
)
