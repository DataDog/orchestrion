// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package utils

import (
	"log"
)

// ExitIfError calls os.Exit(1) if err is not nil
func ExitIfError(err error) {
	if err == nil {
		return
	}
	log.Fatalln(err)
}
