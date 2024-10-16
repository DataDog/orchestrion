// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

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
	// we have a few tests to check the different test statuses (including failures)
	// that's why we don't use the exit code from the tests, but we check the events.
	m.Run()

	// let's check the events inside the CiVisibility payloads
	events := ciVisibilityPayloads.GetEvents()

	// session event
	events.
		CheckEventsByType("test_session_end", 1)

	// module event
	events.
		CheckEventsByType("test_module_end", 1).
		CheckEventsByResourceName("orchestrion/integration/tests/civisibility", 1)

	// test suite event
	events.CheckEventsByType("test_suite_end", 1).
		CheckEventsByResourceName("testing_test.go", 1)

	// test events
	testEvents := events.CheckEventsByType("test", 10)
	normalTests := testEvents.
		CheckEventsByResourceName("testing_test.go.TestNormal", 1).
		CheckEventsByTagAndValue("test.status", "pass", 1)
	failTests := testEvents.
		CheckEventsByResourceName("testing_test.go.TestFail", 1).
		CheckEventsByTagAndValue("test.status", "fail", 1).
		CheckEventsByTagAndValue("error.type", "Fail", 1).
		CheckEventsByTagAndValue("error.message", "failed test", 1)
	errorTests := testEvents.
		CheckEventsByResourceName("testing_test.go.TestError", 1).
		CheckEventsByTagAndValue("test.status", "fail", 1).
		CheckEventsByTagAndValue("error.type", "Error", 1).
		CheckEventsByTagAndValue("error.message", "My error test", 1)
	errorFTests := testEvents.
		CheckEventsByResourceName("testing_test.go.TestErrorf", 1).
		CheckEventsByTagAndValue("test.status", "fail", 1).
		CheckEventsByTagAndValue("error.type", "Errorf", 1).
		CheckEventsByTagAndValue("error.message", "My error test: TestErrorf", 1)
	skipTests := testEvents.
		CheckEventsByResourceName("testing_test.go.TestSkip", 1).
		CheckEventsByTagAndValue("test.status", "skip", 1).
		CheckEventsByTagAndValue("test.skip_reason", "My skipped test", 1)
	skipfTests := testEvents.
		CheckEventsByResourceName("testing_test.go.TestSkipf", 1).
		CheckEventsByTagAndValue("test.status", "skip", 1).
		CheckEventsByTagAndValue("test.skip_reason", "My skipped test: TestSkipf", 1)
	skipNowTests := testEvents.
		CheckEventsByResourceName("testing_test.go.TestSkipNow", 1).
		CheckEventsByTagAndValue("test.status", "skip", 1)
	testWithSubtests := testEvents.
		CheckEventsByResourceName("testing_test.go.TestWithSubTests", 1).
		CheckEventsByTagAndValue("test.status", "pass", 1)
	testWithSubtestsChild1 := testEvents.
		CheckEventsByResourceName("testing_test.go.TestWithSubTests/Sub1", 1).
		CheckEventsByTagAndValue("test.status", "pass", 1)
	testWithSubtestsChild2 := testEvents.
		CheckEventsByResourceName("testing_test.go.TestWithSubTests/Sub2", 1).
		CheckEventsByTagAndValue("test.status", "pass", 1)

	// remaining must be 0
	testEvents.
		Except(
			normalTests,
			failTests,
			errorTests,
			errorFTests,
			skipTests,
			skipfTests,
			skipNowTests,
			testWithSubtests,
			testWithSubtestsChild1,
			testWithSubtestsChild2).
		HasCount(0)

	// All previous checks will cause panic if they fail so we can safely exit with 0 here
	os.Exit(0)
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
