// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/goccy/go-yaml/ast"
)

type oneOf []Point

func OneOf(candidates ...Point) oneOf {
	return candidates
}

func (o oneOf) ImpliesImported() []string {
	// We can only assume a package is imported if all candidates imply it.
	counts := make(map[string]uint)
	for _, jp := range o {
		for _, path := range jp.ImpliesImported() {
			counts[path]++
		}
	}

	total := uint(len(o))
	list := make([]string, 0, len(counts))
	for path, count := range counts {
		if count == total {
			list = append(list, path)
		}
	}
	return list
}

func (o oneOf) PackageMayMatch(ctx *may.PackageContext) may.MatchType {
	sum := may.NeverMatch
	for _, candidate := range o {
		sum = sum.Or(candidate.PackageMayMatch(ctx))
		if sum == may.Match {
			return may.Match
		}
	}
	return sum
}

func (o oneOf) FileMayMatch(ctx *may.FileContext) may.MatchType {
	sum := may.NeverMatch
	for _, candidate := range o {
		sum = sum.Or(candidate.FileMayMatch(ctx))
		if sum == may.Match {
			return may.Match
		}
	}
	return sum
}

func (o oneOf) Matches(ctx context.AspectContext) bool {
	for _, candidate := range o {
		if candidate.Matches(ctx) {
			return true
		}
	}
	return false
}

func (o oneOf) Hash(h *fingerprint.Hasher) error {
	return h.Named("one-of", fingerprint.List[Point](o))
}

func init() {
	unmarshalers["one-of"] = func(ctx gocontext.Context, node ast.Node) (Point, error) {
		var nodes []ast.Node
		if err := yaml.NodeToValueContext(ctx, node, &nodes); err != nil {
			return nil, err
		}

		if len(nodes) == 1 {
			pt, err := FromYAML(ctx, nodes[0])
			return pt, err
		}

		candidates := make([]Point, len(nodes))
		for i, n := range nodes {
			var err error
			if candidates[i], err = FromYAML(ctx, n); err != nil {
				return nil, err
			}
		}
		return OneOf(candidates...), nil
	}
}
