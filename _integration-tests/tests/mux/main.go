// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"orchestrion/integration"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	ping := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = io.WriteString(w, `{"message":"pong"}`)
	}
	r.HandleFunc("/ping", ping).Methods("GET")

	//dd:ignore
	s := &http.Server{
		Addr:    ":8084",
		Handler: r,
	}
	integration.OnSignal(func() {
		ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
		s.Shutdown(ctx)
	})
	log.Print(s.ListenAndServe())
}
