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
)

func main() {
	defer log.Printf("Server shutting down gracefully.")

	s := &http.Server{
		Addr:    ":8085",
		Handler: http.HandlerFunc(handle),
	}
	integration.OnSignal(func() {
		s.Shutdown(context.Background())
	})
	log.Printf("Server shut down: %v", s.ListenAndServe())
}

func handle(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	defer r.Body.Close()
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}
