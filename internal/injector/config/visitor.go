// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"github.com/DataDog/orchestrion/internal/injector/aspect"
)

type (
	Visitor = func(cfg File, pkgPath string) error

	File interface {
		Name() string
		Description() string
		Caveats() string
		Icon() string

		OwnAspects() []*aspect.Aspect
	}
)

// Visit calls the visitor for each configuration found in the specified root
// [Config].
func Visit(cfg Config, visitor Visitor) error {
	return cfg.visit(visitor, "")
}

func (c *configYML) Name() string {
	return c.meta.name
}

func (c *configYML) Description() string {
	return c.meta.description
}

func (c *configYML) Caveats() string {
	return c.meta.caveats
}

func (c *configYML) Icon() string {
	return c.meta.icon
}

func (c *configYML) OwnAspects() []*aspect.Aspect {
	res := make([]*aspect.Aspect, len(c.aspects))
	copy(res, c.aspects)
	return res
}
