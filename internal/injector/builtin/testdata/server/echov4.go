// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
//line <generated>:1
	echo1 "gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4"
)

//line samples/server/echov4.go:14
func echoV4Server() {
	/*dd:instrumented*/ r := echo.New()
//line <generated>:1
	//dd:startinstrument
	{
		r.Use(echo1.Middleware())
	}
	//dd:endinstrument
//line samples/server/echov4.go:16
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
	/*dd:instrumented*/ a.srv = echo.New()
//line <generated>:1
	//dd:startinstrument
	{
		a.srv.Use(echo1.Middleware())
	}
	//dd:endinstrument
//line samples/server/echov4.go:30
	a.srv.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"message": "pong",
		})
	})
	_ = a.srv.Start(":8888")
}
