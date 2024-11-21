// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package builtin contains built-in injection configurations for supported
// instrumentations.
package builtin

//go:generate go run ./generator -i=yaml/*.yml -i=yaml/*/*.yml -p=builtin -y=./all.yml -d=./generated_deps.go -C=1 -docs=../../../_docs/content/docs/built-in/ -schemadocs=../../../_docs/content/contributing/aspects/

import (
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml"
)
