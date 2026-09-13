// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"encoding/xml"
	"os"
)

type payload struct{ Path string }

func main() {
	data, err := xml.Marshal(payload{os.Getenv("TAINT_PATH")})
	if err != nil {
		os.Exit(1)
	}
	_, _ = os.Open(string(data))
}
