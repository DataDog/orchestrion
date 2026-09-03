# Adding a patched-Go taint fixture
Create one directory. Do not edit the suite driver:
```text
fixture/myfixture/
├── main.go
└── cases.json
```
Use `TAINT_PATH` as the source and pass its value to `os.Open` as the sink:
```go
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func main() {
	path := os.Getenv("TAINT_PATH")
	_, _ = os.Open(path)
}
```

Declare one or more globally unique test cases in `cases.json`:

```json
[
  {
    "name": "my fixture",
    "taintPath": "/tmp/iast-my-fixture",
    "dirtyReports": 1,
    "taintEnabled": true,
    "race": false,
    "env": {"EXAMPLE_MODE": "dirty"}
  }
]
```

`dirtyReports` defaults to `0`; `race` and `env` are optional.
