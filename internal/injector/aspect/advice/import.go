// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package advice provides implementations of the injector.Action interface for
// common AST changes.
package advice

import (
	"gopkg.in/yaml.v3"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
)

type addBlankImport string

func AddBlankImport(path string) addBlankImport {
	return addBlankImport(path)
}

func (a addBlankImport) Apply(ctx context.AdviceContext) (bool, error) {
	added := ctx.AddImport(string(a), "_")
	return added, nil
}

func (a addBlankImport) AddedImports() []string {
	return []string{string(a)}
}

func (a addBlankImport) Hash(h *fingerprint.Hasher) error {
	return h.Named("add-blank-import", fingerprint.String(a))
}

func init() {
	unmarshalers["add-blank-import"] = func(node *yaml.Node) (Advice, error) {
		var path string
		if err := node.Decode(&path); err != nil {
			return nil, err
		}
		return AddBlankImport(path), nil
	}
}
