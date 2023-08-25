// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

func gorillaMuxServer() {
	r := mux.NewRouter()
	ping := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = io.WriteString(w, `{"message":"pong"}`)
	}
	r.HandleFunc("/ping", ping).Methods("GET")
	_ = http.ListenAndServe(":8080", r)
}
