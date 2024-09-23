// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"

	"github.com/DataDog/orchestrion/internal/injector/singleton"
	"gopkg.in/yaml.v3"
)

type unmarshalerFn func(*yaml.Node) (Point, error)

var unmarshalers = make(map[string]unmarshalerFn)

func FromYAML(node *yaml.Node) (Point, error) {
	key, value, err := singleton.Unmarshal(node)
	if err != nil {
		return nil, err
	}

	unmarshaller, found := unmarshalers[key]
	if !found {
		return nil, fmt.Errorf("line %d: unknown injection point type %q", node.Line, key)
	}

	ip, err := unmarshaller(value)
	return ip, err
}
