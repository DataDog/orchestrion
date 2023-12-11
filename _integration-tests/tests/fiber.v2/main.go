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
	integration.OnSignal(func() {
		r.Shutdown()
	})
	log.Print(r.Listen(":8089"))
}
