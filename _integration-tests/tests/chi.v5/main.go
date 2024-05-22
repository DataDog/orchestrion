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
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	router := chi.NewRouter()
	//dd:ignore
	s := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: router,
	}

	router.Get("/quit",
		//dd:ignore
		func(w http.ResponseWriter, _ *http.Request) {
			log.Println("Shutdown requested...")
			defer s.Shutdown(context.Background())
			w.Write([]byte("Goodbye\n"))
		})
	router.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Hello World!\n"))
	})

	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})
	log.Print(s.ListenAndServe())
}
