// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"
	"os"
)

func main() {
	source := os.Getenv("TAINT_PATH")
	if source == "" {
		source = "/tmp/iast-fmtsprintfwithmapaggregate"
	}
	values := map[string]string{"path": source}
	value := fmt.Sprintf("path-%v", values)
	_, _ = os.Open(value)
}
