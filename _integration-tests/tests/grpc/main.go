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
)

var s *http.Server

func main() {
	go runServer()

	s = &http.Server{
		Addr:    "127.0.0.1:8083",
		Handler: http.HandlerFunc(handle),
	}
	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})
	log.Printf("Server shut down: %v", s.ListenAndServe())
}

func handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/quit" {
		log.Println("Shutdown requested...")
		defer s.Shutdown(context.Background())
		w.Write([]byte("Goodbye\n"))
		return
	}

	runClient()
	w.WriteHeader(http.StatusOK)
}
