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

type Foo struct{}

func (f Foo) FooHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("Foo!"))
}

func buildHandlers() {
	http.HandleFunc("/foo/bar", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("done!"))
	})
	v := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("another one!"))
	}
	v = v

	type holder struct {
		f http.HandlerFunc
	}

	x := holder{
		f: func(w http.ResponseWriter, request *http.Request) {
			w.Write([]byte("asdf"))
		},
	}

	x = x
	// silly legal things
	func(w http.ResponseWriter, r *http.Request) {
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
			w.Write([]byte("goroutine!"))
		}(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/asfd", nil))
	}

	defer func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("goroutine!"))
	}(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/asfd", nil))
}
