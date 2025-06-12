// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build tools

package instrument

// This merely forwards integrations from the `github.com/DataDog/dd-trace-go/orchestrion/all/v2`
// package. This is only present as a way to ease migrations of the integrations
// to the `dd-trace-go` package.
import (
	_ "github.com/DataDog/dd-trace-go/orchestrion/all/v2" // integration
	_ "github.com/DataDog/orchestrion"                    // integration
)
