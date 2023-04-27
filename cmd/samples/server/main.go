package main

import (
	"github.com/datadog/orchestrion"
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
	//dd:startinstrument
	orchestrion.ReportHTTPServe(w, r, orchestrion.EventStart, "name", "myHandler", "verb", r.Method)
	defer orchestrion.ReportHTTPServe(w, r, orchestrion.EventEnd, "name", "myHandler", "verb", r.Method)
	//dd:endinstrument
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
	//dd:startinstrument
	orchestrion.ReportHTTPServe(w, r, orchestrion.EventStart, "name", "instrumentedHandler", "verb", r.Method)
	defer orchestrion.ReportHTTPServe(w, r, orchestrion.EventEnd, "name", "instrumentedHandler", "verb", r.Method)
	//dd:endinstrument
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
