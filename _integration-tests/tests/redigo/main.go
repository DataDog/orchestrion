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

	"github.com/gomodule/redigo/redis"
)

func main() {
	mux := &http.ServeMux{}
	s := &http.Server{
		Addr:    "127.0.0.1:8089",
		Handler: mux,
	}

	const (
		network = "tcp"
		address = "localhost:6379"
	)
	pool := &redis.Pool{
		Dial:        func() (redis.Conn, error) { return redis.Dial(network, address) },
		DialContext: func(ctx context.Context) (redis.Conn, error) { return redis.DialContext(ctx, network, address) },
		TestOnBorrow: func(c redis.Conn, _ time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	func() {
		client := pool.Get()
		defer client.Close()

		if _, err := client.Do("SET", "test_key", "test_value"); err != nil {
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
			client, err := pool.GetContext(r.Context())
			if err != nil {
				log.Printf("Could not get client: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "%v\n", err)
				return
			}
			defer client.Close()

			if res, err := client.Do("GET", "test_key", r.Context()); err != nil {
				log.Printf("Error: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "%v\n", err)
			} else {
				w.Write(res.([]byte))
			}
		})

	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})

	log.Print(s.ListenAndServe())
}
