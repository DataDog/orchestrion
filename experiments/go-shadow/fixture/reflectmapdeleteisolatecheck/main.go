// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"reflect"
)

func main() {
	values := map[string]string{"dirty": os.Getenv("TAINT_PATH")}
	mapValue := reflect.ValueOf(values)
	mapValue.SetMapIndex(reflect.ValueOf("dirty"), reflect.Value{})
	_, _ = os.Open(values["dirty"])
}
