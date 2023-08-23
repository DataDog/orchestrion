// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"github.com/gin-gonic/gin"
	gintrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin"
)

func GinMiddleware() gin.HandlerFunc {
	// Passing an empty service name until we have a solid
	// and unified mechanism to guess the service name in Orchestrion.
	// gintrace defaults to DD_SERVICE or gin.router as service name.
	return gintrace.Middleware("")
}
