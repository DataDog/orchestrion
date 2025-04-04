// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package singleton

import (
	"context"
	"errors"

	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/goccy/go-yaml/ast"
)

func Unmarshal(ctx context.Context, node ast.Node) (key string, value ast.Node, err error) {
	mapping, ok := node.(*ast.MappingNode)
	if !ok || len(mapping.Values) != 1 {
		err = errors.New("not a singleton mapping")
		return "", nil, err
	}

	if err = yaml.NodeToValueContext(ctx, mapping.Values[0].Key, &key); err != nil {
		return "", nil, err
	}

	return key, mapping.Values[0].Value, nil
}
