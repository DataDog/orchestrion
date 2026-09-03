// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"reflect"
)

type key struct {
	value int
}

func main() {
	values := map[key]string{{value: 1}: os.Getenv("TAINT_PATH")}
	_, _ = os.Open(values[key{value: 1}])

	mapValue := reflect.ValueOf(values)
	mapKey := reflect.ValueOf(key{value: 1})
	mapValue.SetMapIndex(mapKey, reflect.Value{})
	mapValue.SetMapIndex(mapKey, reflect.ValueOf("clean"))
	_, _ = os.Open(values[key{value: 1}])
}
