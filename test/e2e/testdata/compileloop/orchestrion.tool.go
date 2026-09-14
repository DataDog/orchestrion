// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// This file is what `orchestrion pin` would have created, and its presence keeps
// automatic pinning from adding tracer integrations this fixture does not need:
// the only aspect it uses is the one declared in `orchestrion.yml`.

//go:build tools

package tools
