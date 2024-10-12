// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

////go:build integration

package civisibility

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/tinylib/msgp/msgp"
)

var ciVisibilityPayloads mockPayloads

func TestMain(m *testing.M) {
	// let's enable CI Visibility mode
	server := enableCiVisibilityEndpointMock()
	defer server.Close()

	// because CI Visibility mode is enabled all tests are going to be instrumented
	exitCode := m.Run()

	// let's check the events inside the CiVisibility payloads
	events := ciVisibilityPayloads.GetEvents()

	// session event
	events.
		CheckEventsByType("test_session_end", 1).
		ShowResourceNames()

	// module event
	events.
		CheckEventsByType("test_module_end", 1).
		CheckEventsByResourceName("orchestrion/integration/tests/civisibility", 1).
		ShowResourceNames()

	// test suite event
	events.CheckEventsByType("test_suite_end", 1).
		CheckEventsByResourceName("testing_test.go", 1).
		ShowResourceNames()

	// test events
	testEvents := events.CheckEventsByType("test", 2)
	testEvents.
		CheckEventsByResourceName("testing_test.go.TestNormal", 1).
		ShowResourceNames()
	testEvents.
		CheckEventsByResourceName("testing_test.go.TestSkip", 1).
		CheckEventsByTagAndValue("test.status", "skip", 1).
		ShowResourceNames()

	os.Exit(exitCode)
}

func enableCiVisibilityEndpointMock() *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/citestcycle" {
			fmt.Printf("mockapi: test cycle payload received.\n")

			// first we need to read the body
			// then we need to unzip the body
			// then we need to convert the body to json
			// then we need to decode the json

			gzipReader, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer gzipReader.Close()

			// Convert the message pack to json
			jsonBuf := new(bytes.Buffer)
			_, err = msgp.CopyToJSON(jsonBuf, gzipReader)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			var payload mockPayload
			err = json.Unmarshal(jsonBuf.Bytes(), &payload)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			ciVisibilityPayloads = append(ciVisibilityPayloads, &payload)

			// fmt.Println(string(jsonBuf.Bytes()))
			w.WriteHeader(http.StatusAccepted)
			return
		}

		w.WriteHeader(http.StatusBadRequest)
	}))

	fmt.Printf("mockapi: Url: %s\n", server.URL)

	os.Setenv("DD_CIVISIBILITY_ENABLED", "true")
	os.Setenv("DD_TRACE_DEBUG", "true")
	os.Setenv("DD_CIVISIBILITY_AGENTLESS_ENABLED", "true")
	os.Setenv("DD_CIVISIBILITY_AGENTLESS_URL", server.URL)
	os.Setenv("DD_API_KEY", "***")

	return server
}
