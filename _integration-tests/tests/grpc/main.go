// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"log"
	"net/http"

	"orchestrion/integration"
)

func main() {
	go runServer()

	s := &http.Server{
		Addr:    ":8083",
		Handler: http.HandlerFunc(handle),
	}
	integration.OnSignal(func() {
		s.Shutdown(context.Background())
	})
	log.Printf("Server shut down: %v", s.ListenAndServe())
}

func handle(w http.ResponseWriter, r *http.Request) {
	runClient()
	w.WriteHeader(http.StatusOK)
}
