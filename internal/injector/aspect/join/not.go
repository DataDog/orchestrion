// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"gopkg.in/yaml.v3"
)

type not struct {
	JoinPoint Point
}

func Not(jp Point) not {
	return not{jp}
}

func (not) ImpliesImported() []string {
	return nil
}

func (n not) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	return n.JoinPoint.PackageMayMatch(ctx).Not()
}

func (n not) FileMayMatch(ctx *may.FileContext) may.MatchType {
	return n.JoinPoint.FileMayMatch(ctx).Not()
}

func (n not) Matches(ctx context.AspectContext) bool {
	return !n.JoinPoint.Matches(ctx)
}

func (n not) Hash(h *fingerprint.Hasher) error {
	return h.Named("not", n.JoinPoint)
}

func init() {
	unmarshalers["not"] = func(node *yaml.Node) (Point, error) {
		jp, err := FromYAML(node)
		if err != nil {
			return nil, err
		}
		return Not(jp), nil
	}
}
