package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datadog/orchestrion"
)

func main() {
	//dd:startinstrument
	defer orchestrion.Init()()
	//dd:endinstrument
	s := http.NewServeMux()
	//dd:startwrap
	s.HandleFunc("/handle", orchestrion.WrapHandlerFunc(myHandler))
	//dd:endwrap
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	defer r.Body.Close()
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func myClient() {
	//dd:startwrap
	client := orchestrion.WrapHTTPClient(&http.Client{
		Timeout: time.Second,
	})
	//dd:endwrap
	req, err := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "http://localhost:8080",
		strings.NewReader(os.Args[1]))
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.Body == nil {
		return
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
