# Adding an Orchestrion taint E2E case

Add exactly one file named `case_<ID3>_<slug>.go` in this directory. Use the
coverage-ledger ID, or `000` and `ID: 0` when no ledger row applies. Do not edit
the harness or any other case. Names must be unique, stable, and lowercase.

```go
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   21,
		Name: "string concat",
		Run: func() {
			_ = os.Setenv("CASE_021_INPUT", "secret")
			source := os.Getenv("CASE_021_INPUT")
			_, _ = os.Open("prefix-" + source)
		},
		Want: []Expect{{Value: "prefix-secret", Ranges: []taint.Range{{Start: 7, End: 13}}}},
	})
}
```

Keep `os.Getenv` and `os.Open` as direct calls. `Want` is the complete report
set: omit it for no report, use exact ranges when known, or `Ranges: nil` only
when the value must be tainted but its exact range cannot be determined. Run
`go test ./runtime/taint/...`. Never add registry-capacity or saturation cases
here: the registry is process-global, so they would poison every later case.
