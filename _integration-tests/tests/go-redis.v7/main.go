// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"orchestrion/integration"
	"time"

	"github.com/go-redis/redis/v7"
)

func main() {
	mux := &http.ServeMux{}
	s := &http.Server{
		Addr:    "127.0.0.1:8090",
		Handler: mux,
	}

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	func() {
		if err := client.Set("test_key", "test_value", 0).Err(); err != nil {
			log.Fatalf("Failed to insert test data: %v", err)
		}
	}()

	mux.HandleFunc("/quit",
		//dd:ignore
		func(w http.ResponseWriter, r *http.Request) {
			log.Println("Shutdown requested...")
			defer s.Shutdown(context.Background())
			w.Write([]byte("Goodbye\n"))
		})

	mux.HandleFunc("/",
		//dd:ignore
		func(w http.ResponseWriter, r *http.Request) {
			if res, err := client.WithContext(r.Context()).Get("test_key").Result(); err != nil {
				log.Printf("Error: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "%v\n", err)
			} else {
				w.Write([]byte(res))
			}
		})

	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})

	log.Print(s.ListenAndServe())
}
