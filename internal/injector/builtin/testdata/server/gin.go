// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
//line <generated>:1
	gin1 "gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin"
)

//line samples/server/gin.go:14
func ginServer() {
	//dd:instrumented
	r := gin.Default()
//line <generated>:1
	//dd:startinstrument
	{
		r.Use(gin1.Middleware(""))
	}
	//dd:endinstrument
//line samples/server/gin.go:16
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.Run()
}
