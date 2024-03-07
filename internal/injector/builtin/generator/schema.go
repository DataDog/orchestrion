// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"github.com/datadog/orchestrion/internal/injector/aspect"
)

type ConfigurationFile struct {
	Metadata Metadata `yaml:"meta"`
	Aspects  []aspect.Aspect
}

type Metadata struct {
	Name        string
	Description string
	Icon        string `yaml:",omitempty"`
	Caveats     string `yaml:",omitempty"`
}
