package main

import (
	"io"
	"log"
	"net/http"

	"github.com/datadog/orchestrion"
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
	//dd:startinstrument
	r = orchestrion.HandleHeader(r)
	orchestrion.Report(r.Context(), orchestrion.EventStart, "name", "instrumentedHandler", "verb", r.Method)
	defer orchestrion.Report(r.Context(), orchestrion.EventEnd, "name", "instrumentedHandler", "verb", r.Method)
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
