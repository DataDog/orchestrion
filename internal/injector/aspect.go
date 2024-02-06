// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"errors"
	"fmt"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/advice"
	"github.com/datadog/orchestrion/internal/injector/join"
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
}

func (a *Aspect) AsCode() (jp, adv jen.Code) {
	jp = a.JoinPoint.AsCode()
	adv = jen.Index().Qual("github.com/datadog/orchestrion/internal/injector/advice", "Advice").ValuesFunc(func(g *jen.Group) {
		for _, a := range a.Advice {
			g.Line().Add(a.AsCode())
		}
		g.Empty().Line()
	})
	return
}

func (a *Aspect) UnmarshalYAML(node *yaml.Node) error {
	var ti struct {
		JoinPoint yaml.Node            `yaml:"join-point"`
		Advice    yaml.Node            `yaml:"advice"`
		Extra     map[string]yaml.Node `yaml:",inline"`
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
	if len(ti.Extra) != 0 {
		keys := make([]string, 0, len(ti.Extra))
		for key, val := range ti.Extra {
			keys = append(keys, fmt.Sprintf("%q (line %d)", key, val.Line))
		}
		return fmt.Errorf("unexpected keys: %s", strings.Join(keys, ", "))
	}

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
