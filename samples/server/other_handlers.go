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
)

type foo struct{}

func (foo) fooHandler(rw http.ResponseWriter, _ *http.Request) {
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("Foo!"))
}

func buildHandlers() {
	http.HandleFunc("/foo/bar", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Write([]byte("done!"))
	})
	v := func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("another one!"))
	}

	fmt.Printf("%T\n", v)

	type holder struct {
		f http.HandlerFunc
	}

	x := holder{
		f: func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte("asdf"))
		},
	}

	fmt.Println(x)

	// silly legal things
	func(w http.ResponseWriter, _ *http.Request) {
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
		go func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte("goroutine!"))
		}(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/asfd", nil))
	}

	defer func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("goroutine!"))
	}(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/asfd", nil))
}

//dd:span foo:bar type:potato
func myFunc(_ context.Context, name string) {
	fmt.Println(name)
}

//dd:span foo2:bar2 type:request
func myFunc2(name string, _ *http.Request) {
	fmt.Println(name)
}

//dd:span foo3:bar3 type:request span.name:customName
func myFunc3(name string) error {
	_, err := fmt.Println(name)
	return err
}

func registerHandlers() {
	handler := http.HandlerFunc(myHandler)
	http.Handle("/handle-1", handler)
	http.Handle("/hundle-2", http.HandlerFunc(myHandler))
	http.Handle("/hundle-3", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	http.HandleFunc("/handlefunc-1", handler)
	http.HandleFunc("/handlefunc-2", http.HandlerFunc(myHandler))
	http.HandleFunc("/handlefunc-3", func(http.ResponseWriter, *http.Request) {})
	s := http.NewServeMux()
	s.Handle("/handle-mux", handler)
	s.Handle("/handle-mux", http.HandlerFunc(myHandler))
	s.Handle("/handle-mux", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	s.HandleFunc("/handlefunc-1", handler)
	s.HandleFunc("/handlefunc-2", http.HandlerFunc(myHandler))
	s.HandleFunc("/handlefunc-3", func(http.ResponseWriter, *http.Request) {})
	_ = &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
}
