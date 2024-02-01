// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice

import (
	"context"

	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type addComment struct {
	text string
}

func AddComment(text string) *addComment {
	return &addComment{text: text}
}

func (a *addComment) Apply(_ context.Context, csor *dstutil.Cursor) (bool, error) {
	//TODO: This will have offset the line numbers by 1 and needs fixing in preserveLineInfo mode!
	csor.Node().Decorations().Start.Append(a.text)
	return true, nil
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
