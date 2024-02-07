// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package builtin contains built-in injection configurations for supported
// instrumentations.
package builtin

//go:generate go run ./generate -i yaml/*.yml -p builtin -o ./generated.go
