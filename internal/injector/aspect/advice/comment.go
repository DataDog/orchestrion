// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst/dstutil"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type addComment struct {
	text string
}

func AddComment(text string) *addComment {
	return &addComment{text: text}
}

func (a *addComment) Apply(_ context.Context, node *node.Chain, _ *dstutil.Cursor) (bool, error) {
	//TODO: This will have offset the line numbers by 1 and needs fixing in preserveLineInfo mode!
	node.Node.Decorations().Start.Append(a.text)
	return true, nil
}

func (a *addComment) AsCode() jen.Code {
	return jen.Qual(pkgPath, "AddComment").Call(jen.Lit(a.text))
}

func init() {
	unmarshalers["add-comment"] = func(node *yaml.Node) (Advice, error) {
		var text string
		if err := node.Decode(&text); err != nil {
			return nil, err
		}
		return AddComment(text), nil
	}
}
