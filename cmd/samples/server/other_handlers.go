package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/datadog/orchestrion"
)

type Foo struct{}

func (f Foo) FooHandler(rw http.ResponseWriter, req *http.Request) {
	//dd:startinstrument
	orchestrion.ReportHTTPServe(rw, req, orchestrion.EventStart, "name", "FooHandler", "verb", req.Method)
	defer orchestrion.ReportHTTPServe(rw, req, orchestrion.EventEnd, "name", "FooHandler", "verb", req.Method)
	//dd:endinstrument
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("Foo!"))
}

func buildHandlers() {
	http.HandleFunc("/foo/bar", func(writer http.ResponseWriter, request *http.Request) {
		//dd:startinstrument
		orchestrion.ReportHTTPServe(writer, request, orchestrion.EventStart, "name", "/foo/bar", "verb", request.Method)
		defer orchestrion.ReportHTTPServe(writer, request, orchestrion.EventEnd, "name", "/foo/bar", "verb", request.Method)
		//dd:endinstrument
		writer.Write([]byte("done!"))
	})
	v := func(w http.ResponseWriter, r *http.Request) {
		//dd:startinstrument
		orchestrion.ReportHTTPServe(w, r, orchestrion.EventStart, "name", "v", "verb", r.Method)
		defer orchestrion.ReportHTTPServe(w, r, orchestrion.EventEnd, "name", "v", "verb", r.Method)
		//dd:endinstrument
		w.Write([]byte("another one!"))
	}

	fmt.Printf("%T\n", v)

	type holder struct {
		f http.HandlerFunc
	}

	x := holder{
		f: func(w http.ResponseWriter, request *http.Request) {
			//dd:startinstrument
			orchestrion.ReportHTTPServe(w, request, orchestrion.EventStart, "name", "f", "verb", request.Method)
			defer orchestrion.ReportHTTPServe(w, request, orchestrion.EventEnd, "name", "f", "verb", request.Method)
			//dd:endinstrument
			w.Write([]byte("asdf"))
		},
	}

	fmt.Println(x)

	// silly legal things
	func(w http.ResponseWriter, r *http.Request) {
		//dd:startinstrument
		orchestrion.ReportHTTPServe(w, r, orchestrion.EventStart, "name", "anon", "verb", r.Method)
		defer orchestrion.ReportHTTPServe(w, r, orchestrion.EventEnd, "name", "anon", "verb", r.Method)
		//dd:endinstrument
		client := &http.Client{
			Timeout: time.Second,
		}
		//dd:instrumented
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "localhost:8080", strings.NewReader(os.Args[1]))
		//dd:startinstrument
		if req != nil {
			orchestrion.ReportHTTPCall(req, orchestrion.EventCall, "name", req.URL, "verb", req.Method)
			defer orchestrion.ReportHTTPCall(req, orchestrion.EventReturn, "name", req.URL, "verb", req.Method)
		}
		//dd:endinstrument
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
			//dd:startinstrument
			orchestrion.ReportHTTPServe(w, r, orchestrion.EventStart, "name", "anon", "verb", r.Method)
			defer orchestrion.ReportHTTPServe(w, r, orchestrion.EventEnd, "name", "anon", "verb", r.Method)
			//dd:endinstrument
			w.Write([]byte("goroutine!"))
		}(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/asfd", nil))
	}

	defer func(w http.ResponseWriter, r *http.Request) {
		//dd:startinstrument
		orchestrion.ReportHTTPServe(w, r, orchestrion.EventStart, "name", "anon", "verb", r.Method)
		defer orchestrion.ReportHTTPServe(w, r, orchestrion.EventEnd, "name", "anon", "verb", r.Method)
		//dd:endinstrument
		w.Write([]byte("goroutine!"))
	}(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/asfd", nil))
}

//dd:span foo:bar type:potato
func MyFunc(ctx context.Context, name string) {
	//dd:startinstrument
	orchestrion.Report(ctx, orchestrion.EventStart, "name", "MyFunc", "foo", "bar", "type", "potato")
	defer orchestrion.Report(ctx, orchestrion.EventEnd, "name", "MyFunc", "foo", "bar", "type", "potato")
	//dd:endinstrument
	fmt.Println(name)
}

//dd:span foo2:bar2 type:request
func MyFunc2(name string, req *http.Request) {
	//dd:startinstrument
	orchestrion.Report(req.Context(), orchestrion.EventStart, "name", "MyFunc2", "foo2", "bar2", "type", "request")
	defer orchestrion.Report(req.Context(), orchestrion.EventEnd, "name", "MyFunc2", "foo2", "bar2", "type", "request")
	//dd:endinstrument
	fmt.Println(name)
}

//dd:span foo3:bar3 type:request
func MyFunc3(name string) {
	fmt.Println(name)
}
