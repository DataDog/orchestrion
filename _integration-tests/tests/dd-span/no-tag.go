// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build !buildtag

package ddspan

import (
	"context"

	"orchestrion/integration/validator/trace"
)

func (tc *TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Spans{
				{
					Meta: map[string]any{
						"foo": "bar",
					},
					Children: trace.Spans{
						{
							Meta: map[string]any{
								"variant": "notag",
							},
						},
					},
				},
			},
		},
	}
}

//dd:span variant:notag
func tagSpecificSpan(context.Context) string {
	return "Variant NoTag"
}
