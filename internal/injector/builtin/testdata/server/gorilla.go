// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"io"
	"net/http"

	"github.com/gorilla/mux"
//line <generated>:1
	"github.com/datadog/orchestrion/instrument"
	"github.com/datadog/orchestrion/instrument/event"
	mux1 "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
)

//line samples/server/gorilla.go:15
func gorillaMuxServer() {
	r :=
//line <generated>:1
		mux1.WrapRouter(
//line samples/server/gorilla.go:16
			mux.NewRouter())
//line samples/server/gorilla.go:17
	ping := func(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(r.Context(), event.EventStart, "verb", r.Method)
			defer instrument.Report(r.Context(), event.EventEnd, "verb", r.Method)
		}
		//dd:endinstrument
//line samples/server/gorilla.go:18
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = io.WriteString(w, `{"message":"pong"}`)
	}
	r.HandleFunc("/ping", ping).Methods("GET")
	_ = http.ListenAndServe(":8080", r)
}
