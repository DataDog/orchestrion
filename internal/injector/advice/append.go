// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"

	"github.com/datadog/orchestrion/internal/injector/advice/code"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type appendStatements struct {
	template code.Template
}

func AppendStatements(template code.Template) *appendStatements {
	return &appendStatements{template}
}

func (a *appendStatements) Apply(ctx context.Context, csor *dstutil.Cursor) (bool, error) {
	block, err := a.template.CompileBlock(ctx, csor)
	if err != nil {
		return false, err
	}
	csor.InsertAfter(block)

	return true, nil
}

func init() {
	unmarshalers["append-statements"] = func(node *yaml.Node) (Advice, error) {
		var template code.Template
		if err := node.Decode(&template); err != nil {
			return nil, err
		}

		return AppendStatements(template), nil
	}
}
