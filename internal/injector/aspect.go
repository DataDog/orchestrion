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

func (i *Aspect) UnmarshalYAML(node *yaml.Node) error {
	var ti struct {
		JoinPoint yaml.Node            `yaml:"join-point"`
		Advice    []yaml.Node          `yaml:"advice"`
		Extra     map[string]yaml.Node `yaml:",inline"`
	}
	if err := node.Decode(&ti); err != nil {
		return err
	}

	if ti.JoinPoint.Kind == 0 {
		return errors.New("missing required key 'join-point'")
	}
	if ti.Advice == nil {
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
	if i.JoinPoint, err = join.FromYAML(&ti.JoinPoint); err != nil {
		return err
	}

	i.Advice = make([]advice.Advice, len(ti.Advice))
	for idx, node := range ti.Advice {
		if i.Advice[idx], err = advice.FromYAML(&node); err != nil {
			return err
		}
	}

	return nil
}

var (
	_ yaml.Unmarshaler = (*Aspect)(nil)
)
