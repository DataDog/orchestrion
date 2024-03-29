// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func echoV4Server() {
	r := echo.New()
	r.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"message": "pong",
		})
	})
	_ = r.Start(":8080")
}

type api struct {
	srv *echo.Echo
}

func (a *api) echoV4Server() {
	a.srv = echo.New()
	a.srv.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"message": "pong",
		})
	})
	_ = a.srv.Start(":8888")
}
