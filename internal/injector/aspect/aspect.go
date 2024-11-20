// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"errors"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice"
	"github.com/DataDog/orchestrion/internal/injector/aspect/join"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
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

func (a *Aspect) AsCode() (jp jen.Code, adv jen.Code) {
	jp = a.JoinPoint.AsCode()
	adv = jen.Index().Qual("github.com/DataDog/orchestrion/internal/injector/aspect/advice", "Advice").ValuesFunc(func(g *jen.Group) {
		for _, a := range a.Advice {
			g.Line().Add(a.AsCode())
		}
		g.Empty().Line()
	})
	return
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

func (a *Aspect) UnmarshalYAML(node *yaml.Node) error {
	var ti struct {
		JoinPoint      yaml.Node `yaml:"join-point"`
		Advice         yaml.Node `yaml:"advice"`
		ID             string    `yaml:"id"`
		TracerInternal bool      `yaml:"tracer-internal"`
	}
	if err := node.Decode(&ti); err != nil {
		return err
	}

	if ti.JoinPoint.Kind == 0 {
		return errors.New("missing required key 'join-point'")
	}
	if ti.Advice.Kind == 0 {
		return errors.New("missing required key 'advice'")
	}

	a.ID = ti.ID
	a.TracerInternal = ti.TracerInternal

	var err error
	if a.JoinPoint, err = join.FromYAML(&ti.JoinPoint); err != nil {
		return err
	}

	if ti.Advice.Kind == yaml.SequenceNode {
		var nodes []yaml.Node
		if err := ti.Advice.Decode(&nodes); err != nil {
			return err
		}
		a.Advice = make([]advice.Advice, len(nodes))
		for i, node := range nodes {
			a.Advice[i], err = advice.FromYAML(&node)
			if err != nil {
				return err
			}
		}
	} else {
		adv, err := advice.FromYAML(&ti.Advice)
		if err != nil {
			return err
		}
		a.Advice = []advice.Advice{adv}
	}

	return nil
}

var (
	_ yaml.Unmarshaler = (*Aspect)(nil)
)
