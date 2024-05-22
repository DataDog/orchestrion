// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"orchestrion/integration"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	//dd:ignore
	s := &http.Server{
		Addr:    "127.0.0.1:8082",
		Handler: r.Handler(),
	}

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.GET("/quit", func(c *gin.Context) {
		log.Println("Shutdown requested...")
		defer s.Shutdown(context.Background())
		c.JSON(http.StatusOK, gin.H{
			"message": "Goodbye",
		})
	})

	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})
	log.Print(s.ListenAndServe())
}
