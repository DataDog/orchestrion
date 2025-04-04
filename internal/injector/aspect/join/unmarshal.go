// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"context"
	"fmt"

	"github.com/DataDog/orchestrion/internal/injector/singleton"
	"github.com/goccy/go-yaml/ast"
)

type unmarshalerFn func(context.Context, ast.Node) (Point, error)

var unmarshalers = make(map[string]unmarshalerFn)

func FromYAML(ctx context.Context, node ast.Node) (Point, error) {
	key, value, err := singleton.Unmarshal(ctx, node)
	if err != nil {
		return nil, err
	}

	unmarshaller, found := unmarshalers[key]
	if !found {
		return nil, fmt.Errorf("unknown injection point type %q", key)
	}

	ip, err := unmarshaller(ctx, value)
	return ip, err
}
