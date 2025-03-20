// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package traceutil

import (
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
)

type EnvVarCarrier struct {
	Env *[]string
}

var _ tracer.TextMapReader = (*EnvVarCarrier)(nil)
var _ tracer.TextMapWriter = (*EnvVarCarrier)(nil)

const envVarPrefix = "DD_X_"

func (c EnvVarCarrier) ForeachKey(handler func(key string, val string) error) error {
	for _, val := range *c.Env {
		if !strings.HasPrefix(val, envVarPrefix) {
			continue
		}

		key, val, _ := strings.Cut(val, "=")
		key = headerStyle(key)
		if err := handler(key, val); err != nil {
			return err
		}
	}
	return nil
}

func (c EnvVarCarrier) Set(key string, value string) {
	varName := envVarStyle(key)
	for idx, val := range *c.Env {
		if strings.HasPrefix(val, varName+"=") {
			(*c.Env)[idx] = varName + "=" + value
			return
		}
	}
	*c.Env = append(*c.Env, varName+"="+value)
}

func envVarStyle(key string) string {
	key = strings.ToUpper(key)
	return envVarPrefix + strings.ReplaceAll(key, "-", "_")
}

func headerStyle(key string) string {
	key = strings.TrimPrefix(key, envVarPrefix)
	key = strings.ToLower(key)
	return strings.ReplaceAll(key, "_", "-")
}
