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
	"github.com/datadog/orchestrion/instrument/event"
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
	router.Get("/", func(w http.ResponseWriter, __argument__1 *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(__argument__1.Context(), event.EventStart, "verb", __argument__1.Method)
			defer instrument.Report(__argument__1.Context(), event.EventEnd, "verb", __argument__1.Method)
		}
		//dd:endinstrument
//line samples/server/chiv5.go:17
		w.Write([]byte("Hello World!\n"))
	})
	http.ListenAndServe(":8080", router)
}
