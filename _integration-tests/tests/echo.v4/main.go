// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"log"
	"net/http"
	"orchestrion/integration"
	"time"

	"github.com/labstack/echo/v4"
)

func main() {
	r := echo.New()
	r.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"message": "pong",
		})
	})
	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		r.Shutdown(ctx)
	})
	log.Print(r.Start(":8081"))
}
