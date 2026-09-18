// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package dependency

var retained []byte

//go:noinline
func FreshString(value string) string {
	copied := make([]byte, len(value))
	copy(copied, value)
	retained = copied
	return string(copied)
}
