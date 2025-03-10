// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"gopkg.in/yaml.v3"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
)

type allOf []Point

func AllOf(requirements ...Point) allOf {
	return requirements
}

func (o allOf) ImpliesImported() (list []string) {
	for _, jp := range o {
		list = append(list, jp.ImpliesImported()...)
	}
	return
}

func (o allOf) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	sum := may.Match
	for _, candidate := range o {
		sum = sum.And(candidate.PackageMayMatch(ctx))
		if sum == may.NeverMatch {
			return may.NeverMatch
		}
	}
	return sum
}

func (o allOf) FileMayMatch(ctx *may.FileContext) may.MatchType {
	sum := may.Match
	for _, candidate := range o {
		sum = sum.And(candidate.FileMayMatch(ctx))
		if sum == may.NeverMatch {
			return may.NeverMatch
		}
	}
	return sum
}

func (o allOf) Matches(ctx context.AspectContext) bool {
	for _, candidate := range o {
		if !candidate.Matches(ctx) {
			return false
		}
	}
	// Never matches if there is no requirement
	return len(o) > 0
}

func (o allOf) Hash(h *fingerprint.Hasher) error {
	return h.Named("all-of", fingerprint.List[Point](o))
}

func init() {
	unmarshalers["all-of"] = func(node *yaml.Node) (Point, error) {
		var nodes []yaml.Node
		if err := node.Decode(&nodes); err != nil {
			return nil, err
		}

		if len(nodes) == 1 {
			pt, err := FromYAML(&nodes[0])
			return pt, err
		}

		requirements := make([]Point, len(nodes))
		for i, n := range nodes {
			var err error
			if requirements[i], err = FromYAML(&n); err != nil {
				return nil, err
			}
		}
		return AllOf(requirements...), nil
	}
}
