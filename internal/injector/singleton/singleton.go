// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package singleton

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func Unmarshal(node *yaml.Node) (key string, value *yaml.Node, err error) {
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		err = fmt.Errorf("line %d: cannot unmarshal: not a singleton mapping", node.Line)
		return
	}

	if err = node.Content[0].Decode(&key); err != nil {
		return
	}

	value = node.Content[1]

	return
}
