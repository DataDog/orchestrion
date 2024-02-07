// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"
//line <generated>:1
	"github.com/datadog/orchestrion/instrument"
	"github.com/datadog/orchestrion/instrument/event"
)

//line samples/server/other_handlers.go:18
type foo struct{}

func (f foo) fooHandler(rw http.ResponseWriter, req *http.Request) {
//line <generated>:1
	//dd:startinstrument
	{
		instrument.Report(req.Context(), instrument.EventStart, "name", "fooHandler", "verb", req.Method)
		defer instrument.Report(req.Context(), instrument.EventEnd, "name", "fooHandler", "verb", req.Method)
	}
	//dd:endinstrument
//line samples/server/other_handlers.go:21
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("Foo!"))
}

func buildHandlers() {
	http.HandleFunc("/foo/bar", func(writer http.ResponseWriter, request *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(request.Context(), instrument.EventStart, "verb", request.Method)
			defer instrument.Report(request.Context(), instrument.EventEnd, "verb", request.Method)
		}
		//dd:endinstrument
//line samples/server/other_handlers.go:27
		writer.Write([]byte("done!"))
	})
	v := func(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(r.Context(), instrument.EventStart, "verb", r.Method)
			defer instrument.Report(r.Context(), instrument.EventEnd, "verb", r.Method)
		}
		//dd:endinstrument
//line samples/server/other_handlers.go:30
		w.Write([]byte("another one!"))
	}

	fmt.Printf("%T\n", v)

	type holder struct {
		f http.HandlerFunc
	}

	x := holder{
		f: func(w http.ResponseWriter, request *http.Request) {
//line <generated>:1
			//dd:startinstrument
			{
				instrument.Report(request.Context(), instrument.EventStart, "verb", request.Method)
				defer instrument.Report(request.Context(), instrument.EventEnd, "verb", request.Method)
			}
			//dd:endinstrument
//line samples/server/other_handlers.go:41
			w.Write([]byte("asdf"))
		},
	}

	fmt.Println(x)

	// silly legal things
	func(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(r.Context(), instrument.EventStart, "verb", r.Method)
			defer instrument.Report(r.Context(), instrument.EventEnd, "verb", r.Method)
		}
		//dd:endinstrument
//line samples/server/other_handlers.go:49
		client := &http.Client{
			Timeout: time.Second,
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "localhost:8080", strings.NewReader(os.Args[1]))
		if err != nil {
			panic(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		fmt.Println(resp.Status)
		w.Write([]byte("expression!"))
	}(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/asfd", nil))

	for i := 0; i < 10; i++ {
		go func(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
			//dd:startinstrument
			{
				instrument.Report(r.Context(), instrument.EventStart, "verb", r.Method)
				defer instrument.Report(r.Context(), instrument.EventEnd, "verb", r.Method)
			}
			//dd:endinstrument
//line samples/server/other_handlers.go:66
			w.Write([]byte("goroutine!"))
		}(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/asfd", nil))
	}

	defer func(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(r.Context(), instrument.EventStart, "verb", r.Method)
			defer instrument.Report(r.Context(), instrument.EventEnd, "verb", r.Method)
		}
		//dd:endinstrument
//line samples/server/other_handlers.go:71
		w.Write([]byte("goroutine!"))
	}(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/asfd", nil))
}

//dd:span foo:bar type:potato
func myFunc(ctx context.Context, name string) {
//line <generated>:1
	//dd:startinstrument
	{
		instrument.Report(ctx, event.EventStart, "name", "myFunc", "foo", "bar", "type", "potato")
		defer instrument.Report(ctx, event.EventEnd, "name", "myFunc", "foo", "bar", "type", "potato")
	}
	//dd:endinstrument
//line samples/server/other_handlers.go:77
	fmt.Println(name)
}

//dd:span foo2:bar2 type:request
func myFunc2(name string, req *http.Request) {
//line <generated>:1
	//dd:startinstrument
	{
		instrument.Report(req.Context(), event.EventStart, "name", "myFunc2", "foo2", "bar2", "type", "request")
		defer instrument.Report(req.Context(), event.EventEnd, "name", "myFunc2", "foo2", "bar2", "type", "request")
	}
	//dd:endinstrument
//line samples/server/other_handlers.go:82
	fmt.Println(name)
}

//dd:span foo3:bar3 type:request
func myFunc3(name string) {
	fmt.Println(name)
}

func registerHandlers() {
	handler := http.HandlerFunc(myHandler)
	http.Handle("/handle-1", handler)
	http.Handle("/hundle-2", http.HandlerFunc(myHandler))
	http.Handle("/hundle-3", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(r.Context(), instrument.EventStart, "verb", r.Method)
			defer instrument.Report(r.Context(), instrument.EventEnd, "verb", r.Method)
		}
		//dd:endinstrument
	}))
//line samples/server/other_handlers.go:95
	http.HandleFunc("/handlefunc-1", handler)
	http.HandleFunc("/handlefunc-2", http.HandlerFunc(myHandler))
	http.HandleFunc("/handlefunc-3", func(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(r.Context(), instrument.EventStart, "verb", r.Method)
			defer instrument.Report(r.Context(), instrument.EventEnd, "verb", r.Method)
		}
		//dd:endinstrument
	})
//line samples/server/other_handlers.go:98
	s := http.NewServeMux()
	s.Handle("/handle-mux", handler)
	s.Handle("/handle-mux", http.HandlerFunc(myHandler))
	s.Handle("/handle-mux", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(r.Context(), instrument.EventStart, "verb", r.Method)
			defer instrument.Report(r.Context(), instrument.EventEnd, "verb", r.Method)
		}
		//dd:endinstrument
	}))
//line samples/server/other_handlers.go:102
	s.HandleFunc("/handlefunc-1", handler)
	s.HandleFunc("/handlefunc-2", http.HandlerFunc(myHandler))
	s.HandleFunc("/handlefunc-3", func(w http.ResponseWriter, r *http.Request) {
//line <generated>:1
		//dd:startinstrument
		{
			instrument.Report(r.Context(), instrument.EventStart, "verb", r.Method)
			defer instrument.Report(r.Context(), instrument.EventEnd, "verb", r.Method)
		}
		//dd:endinstrument
	})
//line samples/server/other_handlers.go:105
	_ = &http.Server{
		Addr: ":8080",
		Handler:
//line <generated>:1
		//dd:startwrap
		instrument.WrapHandler(
//line samples/server/other_handlers.go:107
			handler),
		//dd:endwrap
	}
}
