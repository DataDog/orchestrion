// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"orchestrion/integration/validator/trace"
	"os"
)

func main() {
	var outputJson bool
	flag.BoolVar(&outputJson, "json", false, "Output as JSON")
	flag.Parse()

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalln(err)
	}

	var traces []*trace.Span
	if err := trace.ParseRaw(data, &traces); err != nil {
		log.Fatalln(err)
	}

	if outputJson {
		toEncode := make([]map[string]any, len(traces))
		for i, trace := range traces {
			toEncode[i] = asMap(trace)
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		encoder.Encode(toEncode)
	} else {
		for i, trace := range traces {
			if i > 0 {
				fmt.Println()
			}
			fmt.Println(trace)
		}
	}
}

func asMap(span *trace.Span) map[string]any {
	res := make(map[string]any)

	if span.ID != 0 {
		res["span_id"] = span.ID
	}
	for tag, value := range span.Tags {
		res[tag] = value
	}
	if len(span.Meta) > 0 {
		res["meta"] = span.Meta
	}
	if len(span.Children) > 0 {
		children := make([]map[string]any, len(span.Children))
		for i, child := range span.Children {
			children[i] = asMap(child)
		}
		res["_children"] = children
	}
	return res
}
