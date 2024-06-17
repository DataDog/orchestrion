// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"orchestrion/integration"
	"time"
)

var s *http.Server

func main() {
	defer log.Printf("Server shutting down gracefully.")

	mux := http.NewServeMux()
	s = &http.Server{
		Addr:    "127.0.0.1:8085",
		Handler: mux,
	}

	mux.HandleFunc("/quit",
		func(w http.ResponseWriter, r *http.Request) {
			log.Println("Shutdown requested...")
			defer s.Shutdown(context.Background())
			w.Write([]byte("Goodbye\n"))
			return
		})

	mux.HandleFunc("/hit",
		func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			b, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write(b)
		})

	mux.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			resp, err := http.Post(fmt.Sprintf("http://%s/hit", s.Addr), "text/plain", r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}
			defer resp.Body.Close()

			b, err := io.ReadAll(resp.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

			w.WriteHeader(resp.StatusCode)
			w.Write(b)
		})

	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})
	log.Printf("Server shut down: %v", s.ListenAndServe())
}
