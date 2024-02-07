// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"io"
	"log"
	"net/http"
//line <generated>:1
	"github.com/datadog/orchestrion/instrument"
)

//line samples/server/main.go:14
func main() {
	s := &http.Server{
		Addr: ":8080",
		Handler:
//line <generated>:1
		//dd:startwrap
		instrument.WrapHandler(
//line samples/server/main.go:17
			http.HandlerFunc(myHandler)),
		//dd:endwrap
	}

//line samples/server/main.go:20
	log.Fatal(s.ListenAndServe())
}

// myHandler comment on function
func myHandler(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
	//dd:startinstrument
	{
		instrument.Report(r.Context(), instrument.EventStart, "name", "myHandler", "verb", r.Method)
		defer instrument.Report(r.Context(), instrument.EventEnd, "name", "myHandler", "verb", r.Method)
	}
	//dd:endinstrument
//line samples/server/main.go:25
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
//line <generated>:1
	//dd:startinstrument
	{
		instrument.Report(r.Context(), instrument.EventStart, "name", "instrumentedHandler", "verb", r.Method)
		defer instrument.Report(r.Context(), instrument.EventEnd, "name", "instrumentedHandler", "verb", r.Method)
	}
	//dd:endinstrument
//line samples/server/main.go:38
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
