// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"github.com/gofiber/fiber/v2"
	fibertrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2"
)

func fiberV2Server() {
	//dd:instrumented
	r := fiber.New()
//line <generated>:1
	//dd:startinstrument
	{
		r.Use(fibertrace.Middleware())
	}
	//dd:endinstrument
//line samples/server/fiberv2.go:15
	r.Use(fibertrace.Middleware())
	r.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(map[string]any{
			"message": "pong",
		})
	})
	_ = r.Listen(":8080")
}
