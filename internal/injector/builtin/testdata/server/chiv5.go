// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
//line <generated>:1
	"github.com/datadog/orchestrion/instrument"
	chi1 "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5"
)

//line samples/server/chiv5.go:14
func chiV5Server() {
	//dd:instrumented
	router := chi.NewRouter()
//line <generated>:1
	//dd:startinstrument
	{
		router.Use(chi1.Middleware())
	}
	//dd:endinstrument
//line samples/server/chiv5.go:16
	router.Get("/",
//line <generated>:1
		instrument.WrapHandlerFunc(
//line samples/server/chiv5.go:16
			func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte("Hello World!\n"))
			}))
//line samples/server/chiv5.go:19
	http.ListenAndServe(":8080", router)
}
