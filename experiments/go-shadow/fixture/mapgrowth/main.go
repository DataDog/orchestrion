// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"strconv"
)

func main() {
	values := make(map[string]string)
	values["dirty"] = os.Getenv("TAINT_PATH")
	for index := 0; index < 1024; index++ {
		values[strconv.Itoa(index)] = "clean"
	}
	_, _ = os.Open(values["dirty"])
}
