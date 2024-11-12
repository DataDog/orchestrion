// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build tools

package instrument

// Ensures that `orchestrion.tool.go` files importing this package include all
// the necessary transitive dependencies for all possible integrations, and
// correctly discover the aspects to inject.
import _ "github.com/DataDog/orchestrion/internal/injector/builtin"
