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

	fmt.Printf("%T\n", v)

	type holder struct {
		f http.HandlerFunc
	}

	x := holder{
		f: func(w http.ResponseWriter, request *http.Request) {
			w.Write([]byte("asdf"))
		},
	}

	fmt.Println(x)

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

//dd:span foo:bar type:potato
func MyFunc(ctx context.Context, name string) {
	fmt.Println(name)
}

//dd:span foo2:bar2 type:request
func MyFunc2(name string, req *http.Request) {
	fmt.Println(name)
}

//dd:span foo3:bar3 type:request
func MyFunc3(name string) {
	fmt.Println(name)
}

func registerHandlers() {
	handler := http.HandlerFunc(myHandler)
	//dd:startinstrument
	http.Handle("/handle-1", orchestrion.WrapHandler(handler))
	//dd:endinstrument
	//dd:startinstrument
	http.Handle("/hundle-2", orchestrion.WrapHandler(http.HandlerFunc(myHandler)))
	//dd:endinstrument
	//dd:startinstrument
	http.Handle("/hundle-3", orchestrion.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	//dd:endinstrument
	//dd:startinstrument
	http.HandleFunc("/handlefunc-1", orchestrion.WrapHandlerFunc(handler))
	//dd:endinstrument
	//dd:startinstrument
	http.HandleFunc("/handlefunc-2", orchestrion.WrapHandlerFunc(http.HandlerFunc(myHandler)))
	//dd:endinstrument
	//dd:startinstrument
	http.HandleFunc("/handlefunc-3", orchestrion.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	//dd:endinstrument
}
