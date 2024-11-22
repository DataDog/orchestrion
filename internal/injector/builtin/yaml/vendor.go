// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package vendor

import (
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/api"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/civisibility"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/cloud"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/databases"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/datastreams"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/directive"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/graphql"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/http"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/logs"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/rpc"
	_ "github.com/DataDog/orchestrion/internal/injector/builtin/yaml/stdlib"
)
