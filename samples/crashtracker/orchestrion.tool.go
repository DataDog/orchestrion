// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product contains software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// This file sets up the tracer integrations enabled for this sample. The
// crashtracker integration is opt-in: dd-trace-go does not include it in
// `orchestrion/all`, so importing it here is what enables it.

//go:build tools

package tools

import (
	// Provides integrations for essential `orchestrion` features.
	_ "github.com/DataDog/orchestrion/instrument" // integration

	// The crashtracker integration is not part of `orchestrion/all`; importing
	// it here opts this sample into it.
	_ "github.com/DataDog/dd-trace-go/v2/crashtracker" // integration
)
