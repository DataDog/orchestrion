// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"runtime/pprof"
	"time"
)

func main() {
	// Start CPU profiling if requested
	if profilePath := os.Getenv("CPUPROFILE"); profilePath != "" {
		f, err := os.Create(profilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
		fmt.Println("CPU profiling enabled, writing to:", profilePath)
	}

	// Simple HTTP server with some computational work
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/compute", handleCompute)

	port := "8080"
	fmt.Printf("Starting server on :%s\n", port)
	fmt.Println("Try: curl http://localhost:8080/compute")

	server := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Auto-shutdown after profile collection
	if os.Getenv("CPUPROFILE") != "" {
		go func() {
			time.Sleep(3 * time.Second)
			fmt.Println("Profile collection complete, shutting down...")
			server.Shutdown(context.Background())
		}()

		// Generate some load for profiling
		go generateLoad()
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Server stopped gracefully")
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "PGO Sample Application\n")
	fmt.Fprintf(w, "Visit /compute for computational work\n")
}

func handleCompute(w http.ResponseWriter, r *http.Request) {
	result := doComputationalWork()
	fmt.Fprintf(w, "Computed result: %d\n", result)
}

func doComputationalWork() int64 {
	// Simulate some CPU-intensive work
	var sum int64
	for i := 0; i < 1000000; i++ {
		sum += int64(fibonacci(20))
		sum += int64(rand.Intn(100))
	}
	return sum
}

func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

func generateLoad() {
	time.Sleep(100 * time.Millisecond) // Wait for server to start
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < 50; i++ {
		resp, err := client.Get("http://localhost:8080/compute")
		if err == nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
}

