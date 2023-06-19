// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package server

import (
	"io"
	"log"
	"net/http"
)

func main() {
	s := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(myHandler),
	}

	log.Fatal(s.ListenAndServe())
}

// myHandler comment on function
func myHandler(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	// test comment in function
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	defer r.Body.Close()
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func instrumentedHandler(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	// test comment in function
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	defer r.Body.Close()
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

// comment that is just hanging out unattached
