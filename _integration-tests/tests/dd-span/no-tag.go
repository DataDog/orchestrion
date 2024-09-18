// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration && !buildtag

package ddspan

import (
	"orchestrion/integration/validator/trace"
)

func (*TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name": "spanFromHTTPRequest",
					},
					Meta: map[string]any{
						"function-name": "spanFromHTTPRequest",
						"foo":           "bar",
					},
					Children: trace.Spans{
						{
							Tags: map[string]any{
								"name": "tagSpecificSpan",
							},
							Meta: map[string]any{
								"function-name": "tagSpecificSpan",
								"variant":       "notag",
							},
						},
					},
				},
			},
		},
	}
}

//dd:span variant:notag
func tagSpecificSpan() (string, error) {
	return "Variant NoTag", nil
}
