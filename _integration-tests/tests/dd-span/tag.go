// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build buildtag

package main

import "context"

//dd:span variant:tag
func tagSpecificSpan(context.Context) string {
	return "Variant Tag"
}
