// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"github.com/gofiber/fiber/v2"
//line <generated>:1
	fiber1 "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2"
)

//line samples/server/fiberv2.go:12
func fiberV2Server() {
	//dd:instrumented
	r := fiber.New()
//line <generated>:1
	//dd:startinstrument
	{
		r.Use(fiber1.Middleware())
	}
	//dd:endinstrument
//line samples/server/fiberv2.go:14
	r.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(map[string]any{
			"message": "pong",
		})
	})
	_ = r.Listen(":8080")
}
