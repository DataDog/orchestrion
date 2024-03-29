// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"github.com/gofiber/fiber/v2"
)

func fiberV2Server() {
	r := fiber.New()
	r.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(map[string]any{
			"message": "pong",
		})
	})
	_ = r.Listen(":8080")
}
