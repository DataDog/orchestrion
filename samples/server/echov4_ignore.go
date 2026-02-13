// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func echoV4IgnoreDirectiveServer() {
	r := echo.New()
	handlers := &healthHandlers{}

	r.GET("/ready", handlers.readinessProbe)
	r.GET("/live", handlers.livenessProbe)

	_ = r.Start(":8081")
}

type healthHandlers struct{}

//orchestrion:ignore
func (*healthHandlers) readinessProbe(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

func (*healthHandlers) livenessProbe(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
