// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/singleton"
	"gopkg.in/yaml.v3"
)

type unmarshalerFn func(*yaml.Node) (Advice, error)

var unmarshalers = make(map[string]unmarshalerFn)

func FromYAML(node *yaml.Node) (Advice, error) {
	key, value, err := singleton.Unmarshal(node)
	if err != nil {
		return nil, err
	}

	unmarshaler, ok := unmarshalers[key]
	if !ok {
		return nil, fmt.Errorf("line %d: unknown action type: %q", node.Line, key)
	}

	act, err := unmarshaler(value)
	return act, err
}
