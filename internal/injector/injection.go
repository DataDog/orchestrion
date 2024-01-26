// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"errors"
	"fmt"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/ast"
	"github.com/datadog/orchestrion/internal/injector/at"
	"gopkg.in/yaml.v3"
)

type Injection struct {
	// Point determines whether the injection should be performed on a given node or not.
	Point at.InjectionPoint
	// Actions is the set of actions to use for performing the actual injection.
	Actions []ast.Action
}

func (i *Injection) UnmarshalYAML(node *yaml.Node) error {
	var ti struct {
		Point   yaml.Node            `yaml:"point"`
		Actions []yaml.Node          `yaml:"actions"`
		Extra   map[string]yaml.Node `yaml:",inline"`
	}
	if err := node.Decode(&ti); err != nil {
		return err
	}

	if ti.Point.Kind == 0 {
		return errors.New("missing required key 'point'")
	}
	if ti.Actions == nil {
		return errors.New("missing required key 'actions'")
	}
	if len(ti.Extra) != 0 {
		keys := make([]string, 0, len(ti.Extra))
		for key, val := range ti.Extra {
			keys = append(keys, fmt.Sprintf("%q (line %d)", key, val.Line))
		}
		return fmt.Errorf("unexpected keys: %s", strings.Join(keys, ", "))
	}

	var err error
	if i.Point, err = at.Unmarshal(&ti.Point); err != nil {
		return err
	}

	i.Actions = make([]ast.Action, len(ti.Actions))
	for idx, node := range ti.Actions {
		if i.Actions[idx], err = ast.Unmarshal(&node); err != nil {
			return err
		}
	}

	return nil
}

var (
	_ yaml.Unmarshaler = (*Injection)(nil)
)
