// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"github.com/labstack/echo/v4"
	echotrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4"
)

func EchoV4Middleware() echo.MiddlewareFunc {
	// As in GinMiddleware, passing an empty service name until we have
	// a unified mechanism to guess the service name in Orchestrion.
	// echotrace defaults to DD_SERVICE or echo as service name.
	return echotrace.Middleware()
}
