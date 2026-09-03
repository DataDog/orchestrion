// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"maps"
	"os"
)

func sink(path string) {
	_, _ = os.Open(path)
}

func main() {
	values := map[string]string{"dirty": os.Getenv("TAINT_PATH")}

	sink(values["dirty"])
	if value, ok := values["dirty"]; ok {
		sink(value)
	}
	missing, _ := values["missing"]
	sink(missing)

	for key, value := range values {
		if key == "dirty" {
			sink(value)
		}
	}

	cloned := maps.Clone(values)
	sink(cloned["dirty"])

	values["dirty"] = "clean"
	sink(values["dirty"])
	values["dirty"] = os.Getenv("TAINT_PATH")
	delete(values, "dirty")
	sink(values["dirty"])
	values["dirty"] = os.Getenv("TAINT_PATH")
	clear(values)
	sink(values["dirty"])
}
