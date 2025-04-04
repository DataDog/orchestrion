// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"errors"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice"
	"github.com/DataDog/orchestrion/internal/injector/aspect/join"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/goccy/go-yaml/ast"
)

// Aspect binds advice.Advice to a join.Point, effectively defining a complete
// code injection.
type Aspect struct {
	// JoinPoint determines whether the injection should be performed on a given node or not.
	JoinPoint join.Point
	// Advice is the set of actions to use for performing the actual injection.
	Advice []advice.Advice
	// TracerInternal determines whether the aspect can be woven into the tracer's internal code.
	TracerInternal bool
	// ID is the identifier of the aspect within its configuration file.
	ID string
}

func (a *Aspect) Hash(h *fingerprint.Hasher) error {
	return h.Named(
		"aspect",
		fingerprint.String(a.ID),
		fingerprint.Bool(a.TracerInternal),
		a.JoinPoint,
		fingerprint.List[advice.Advice](a.Advice),
	)
}

func (a *Aspect) AddedImports() (imports []string) {
	// "unsafe" is always implied, because it's special-cased in the go toolchain, and is not a "normal" module.
	implied := map[string]struct{}{"unsafe": {}}
	for _, path := range a.JoinPoint.ImpliesImported() {
		implied[path] = struct{}{}
	}

	for _, adv := range a.Advice {
		for _, path := range adv.AddedImports() {
			if _, implied := implied[path]; implied {
				continue
			}
			imports = append(imports, path)
		}
	}
	return
}

// InjectedPaths returns the list of import paths that may be injected by the
// supplied list of aspects. The output list is not sorted in any particular way
// but does not contain duplicted entries.
func InjectedPaths(list []*Aspect) []string {
	var res []string
	dedup := make(map[string]struct{})

	for _, a := range list {
		for _, path := range a.AddedImports() {
			if _, dup := dedup[path]; dup {
				continue
			}
			dedup[path] = struct{}{}
			res = append(res, path)
		}
	}

	return res
}

func (a *Aspect) UnmarshalYAML(ctx context.Context, node ast.Node) error {
	var ti struct {
		JoinPoint      ast.Node `yaml:"join-point"`
		Advice         ast.Node `yaml:"advice"`
		ID             string   `yaml:"id"`
		TracerInternal bool     `yaml:"tracer-internal"`
	}
	if err := yaml.NodeToValueContext(ctx, node, &ti); err != nil {
		return err
	}

	if ti.JoinPoint == nil {
		return errors.New("missing required key 'join-point'")
	}
	if ti.Advice == nil {
		return errors.New("missing required key 'advice'")
	}

	a.ID = ti.ID
	a.TracerInternal = ti.TracerInternal

	var err error
	if a.JoinPoint, err = join.FromYAML(ctx, ti.JoinPoint); err != nil {
		return err
	}

	if seq, ok := ti.Advice.(*ast.SequenceNode); ok {
		var nodes []ast.Node
		if err := yaml.NodeToValueContext(ctx, seq, &nodes); err != nil {
			return err
		}
		a.Advice = make([]advice.Advice, len(nodes))
		for i, node := range nodes {
			a.Advice[i], err = advice.FromYAML(ctx, node)
			if err != nil {
				return err
			}
		}
	} else {
		adv, err := advice.FromYAML(ctx, ti.Advice)
		if err != nil {
			return err
		}
		a.Advice = []advice.Advice{adv}
	}

	return nil
}

var (
	_ yaml.NodeUnmarshalerContext = (*Aspect)(nil)
)
