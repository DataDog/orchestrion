# Integration Tests

The integration tests in this directory run programs instrumented with orchestrion, and compare the traces received by the Trace Agent to a list of expected traces.

Each test is a directory within the `tests` directory. This directory must contain a go program compilable with `go build`. Orchestrion is run on the source before compiling.

## Running Tests Locally

Tests can be run locally by running the `integration-tests.sh` script in the repository root. 

| :exclamation:  Note: The `integration-tests.sh` script modifies the source of the test applications using Orchestrion. Make sure you have any changes saved before running the test suite. |
|-----------------------------------------|

The script optionally accepts the name of a specific test to run. Otherwise it will run all of the integration tests in the suite.


```
$ ./integration-tests.sh
... Runs all integration tests
```

```
$ ./integration-tests.sh net_http
... Runs the net_http unit test
```



## Creating a New Test

Creating a new test can be done by adding a directory to the `tests` directory. Conventionally, these should be named the same thing as the integrations they test, when adding a test for a new integration.

A test must consist of 2 things:
- A set of `.go` files which build into an executable program. Orchestrion will be run on these files before compiling to add instrumentation. Such programs must be running an HTTP server, and must trigger the desired trace-generating behavior via a web request to some url. This allows the test harness to trigger the test.
- A `validation.json` file, containing the URL that will trigger the trace generation, and a list of traces to be expected in the output.

### `validation.json` structure

```
top level:
{
	"url": "http://localhost:8080",
	"output": [span]
}

span:
{
	"name": "span name",
	"service": "span service",
	"resource": "span resource",
	"type": "span type",
	"meta": {string -> string},
	"metrics": {string -> float},
	"_children": [span]
}
```

#### Example:
```
{
	"url": "http://localhost:8080",
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
					"meta": {,
						"tag": "value"
					}
				}
			]
	}]
}
```
