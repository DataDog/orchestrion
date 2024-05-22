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
	"os"
	"runtime"
	"syscall"
	"time"
)

func main() {
	defer log.Printf("Server shutting down gracefully.")

	s := &http.Server{
		Addr:    "127.0.0.1:8086",
		Handler: http.HandlerFunc(handle),
	}
	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})
	log.Printf("Server shut down: %v", s.ListenAndServe())
}

//dd:span test1:subfn
func subfn(ctx context.Context) {
	log.Printf("Nothing really to do here.")
	runtime.KeepAlive(ctx)
}

func handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/quit" {
		log.Println("Shutdown requested...")
		defer syscall.Kill(os.Getpid(), syscall.SIGTERM)
		w.Write([]byte("Goodbye\n"))
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	defer r.Body.Close()
	w.WriteHeader(http.StatusOK)
	w.Write(b)
	subfn(r.Context())
}
