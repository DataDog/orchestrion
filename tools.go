// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build tools

package tools

import (
	_ "github.com/datadog/orchestrion/internal/injector/aspect/advice/code/generator"
	_ "github.com/datadog/orchestrion/internal/injector/builtin/generator"
	_ "golang.org/x/tools/cmd/stringer"
)
