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
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	//dd:ignore
	s := &http.Server{
		Addr:    ":8080",
		Handler: r.Handler(),
	}
	integration.OnSignal(func() {
		ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
		s.Shutdown(ctx)
	})
	log.Print(s.ListenAndServe())
}
