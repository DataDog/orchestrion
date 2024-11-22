// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
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

func (o oneOf) Matches(ctx context.AspectContext) bool {
	for _, candidate := range o {
		if candidate.Matches(ctx) {
			return true
		}
	}
	return false
}

func (o oneOf) AsCode() jen.Code {
	if len(o) == 1 {
		return o[0].AsCode()
	}

	return jen.Qual(pkgPath, "OneOf").CallFunc(func(g *jen.Group) {
		if len(o) > 0 {
			for _, candidate := range o {
				g.Line().Add(candidate.AsCode())
			}
			g.Line().Empty()
		}
	})
}

func (o oneOf) Hash(h *fingerprint.Hasher) error {
	return h.Named("one-of", fingerprint.List[Point](o))
}

func init() {
	unmarshalers["one-of"] = func(node *yaml.Node) (Point, error) {
		var nodes []yaml.Node
		if err := node.Decode(&nodes); err != nil {
			return nil, err
		}

		if len(nodes) == 1 {
			pt, err := FromYAML(&nodes[0])
			return pt, err
		}

		candidates := make([]Point, len(nodes))
		for i, n := range nodes {
			var err error
			if candidates[i], err = FromYAML(&n); err != nil {
				return nil, err
			}
		}
		return OneOf(candidates...), nil
	}
}
