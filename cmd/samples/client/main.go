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
	if len(os.Args) < 2 {
		return
	}
	client := &http.Client{
		Timeout: time.Second,
	}
	//dd:instrumented
	req, err := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "http://localhost:8080",
		strings.NewReader(os.Args[1]))
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
