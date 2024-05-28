# Integration Tests

The integration tests in this directory run programs instrumented with orchestrion, and compare the traces received by
the Trace Agent to a list of expected traces.

Each test is a directory within the `tests` directory. This directory must contain a go program compilable with
`orchestrion go build`.

## Running Tests Locally

Tests can be run locally by running the `integration-tests.ps1` script in the repository root. This requires a recent
PowerShell interpreter to be available.

```console
$ ./integration-tests.ps1
... Runs all integration tests
```

Optionally, you may specify one or more specific tests to execute by passing their name (case-sensitive) to the script:

```console
$ ./integration-tests.ps1 chi.v5 mux
... Runs only the chi.v5 and mux tests
```


## Creating a New Test

Creating a new test can be done by adding a directory to the `tests` directory. Conventionally, these should be named
the same thing as the integrations they test, when adding a test for a new integration.

A test must consist of 2 things:
- A set of `.go` files which build into an executable program. Orchestrion will be run on these files before compiling
  to add instrumentation. Such programs must be running an HTTP server, and must trigger the desired trace-generating
  behavior via a web request to some url. This allows the test harness to trigger the test.
  * One of the web server's endpoints must trigger a graceful shut-down of the server and termination of the process,
	  ensuring all outstanding trace information is submitted to the agent.
- A `validation.json` file, containing the URL that will trigger the trace generation, and a list of traces to be
  expected in the output.

### `validation.json` structure

```jsonc
// top level:
{
  /* EITHER */
  "url": "http://localhost:8080",       // A URL to send a GET request to
  /* OR */
  "curl": "curl -X POST 'http://localhost:8080'", // A cURL command to use

  /* Regardless of the above: */
  "quit": "http://localhost:8080/quit", // The URL to hit to cleanly terminate the server
  "output": [/* span */]                // The expected spans to find in the traces
}

// span:
{
  "name": "span name",
  "service": "span service",
  "resource": "span resource",
  "type": "span type",
  "meta": { /* string -> string */ },
  "metrics": { /* string -> float */ },
  "_children": [ /* span */]
}
```

#### Example:
```json
{
  "url": "http://localhost:8080",
  "quit": "http://localhost:8080/quit",
  "output": [
    {
      "name": "parent",
      "service": "myservice",
      "resource": "parent resource",
      "type": "parent type",
      "_children": [
        {
          "name": "child",
          "service": "myservice",
          "resource": "child resource",
          "meta": {
            "tag": "value"
          }
        }
      ]
    }
  ]
}
```
