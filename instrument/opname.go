// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

func getOpName(metadata ...any) string {
	rank := map[string]int{
		"verb":          1,
		"function-name": 2,
	}

	var (
		opname string
		oprank = 10_000 // just a higher number than any key in the rank map.
	)
	for i := 0; i < len(metadata); i += 2 {
		if i+1 >= len(metadata) {
			break
		}
		if k, ok := metadata[i].(string); ok {
			if r, ok := rank[k]; ok && r < oprank {
				if on, ok := metadata[i+1].(string); ok {
					opname = on
					oprank = r
					continue
				}
			}
		}
	}
	return opname
}
