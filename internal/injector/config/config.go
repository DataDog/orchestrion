// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package config provides an interface to read an injector configuration from a
// go package on disk; resolving all downstream configuration as necessary.
package config

import (
	"github.com/DataDog/orchestrion/internal/injector/aspect"
)

type Config interface {
	// Aspects returns all aspects declared in this [Config].
	Aspects() []aspect.Aspect
	// Empty returns true if this [Config] contains no aspect.
	Empty() bool
}
