// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"log"
	"orchestrion/integration"

	"github.com/gofiber/fiber/v2"
)

func main() {
	r := fiber.New()
	r.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(map[string]any{
			"message": "pong",
		})
	})
	r.Get("/quit", func(c *fiber.Ctx) error {
		log.Println("Shutdown requested...")
		defer r.Shutdown()
		return c.JSON(map[string]any{
			"message": "Goodbye",
		})
	})
	integration.OnSignal(func() {
		r.Shutdown()
	})
	log.Print(r.Listen(":8089"))
}
