## Contributing

Thanks for your interest in contributing! This is an open source project, so we appreciate community contributions.

Pull requests for bug fixes are welcome, but before submitting new features or changes to current functionalities
[open an issue][new-issue] and discuss your ideas or propose the changes you wish to make. After a resolution is reached
a PR can be submitted for review. PRs created before a decision has been reached may be closed.

### License

Orchestrion is licensed under the [`Apache-2.0` license](/LICENSE). By sumitting a PR to this repository, you are making
the contribution under the terms of the [`Apache-2.0` license](/LICENSE), and that you are authorized to do so.

All files in this repository must include the appropriate `Apache-2.0` license header:

```
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
```

Code that is copied from another repository should be placed in separate files that inly contain code from the same
origin, and that must include the licensing header from the original repository below Datadog's `Apache-2.0` header.
That code must be governed by a license that is compatible with the terms of [`Apache-2.0` license](/LICENSE).

### Pull Request

Orchestrion uses the [conventional commits][conventional-commits] specification for commit messages. Pull requests are
squash-merged, and the PR title and body are used as the commit title and message that will land on the `main` branch.

Please make sure your PR titles follow the [conventional commits][conventional-commits] specification.
In particular, bug fix PR titles should refer to the bug being fixed, not how it is fixed:

- :x: `fix: infer -coverpkg argument if absent`
- :white_check_mark: `fix: link step fails if -cover is used without -coverpkg`

We expect the PR body for most changes should answer the following questions:

- Why is the change being made?
  - For bug fixes, this usually means providing more detail about the bug's root cause
  - For enhancements, this means explaining the use-case or added value for the change
- What is the change?
  - Explain what's changed in plain english; this will help PR reviewers make sense of the code changes
  - Allows people to make sense of the commit without having to read or understand the code

Link to any relevant GitHub issue (including in other repositories) by using header-style footers in your PR body, e.g:

- `Fixes: #123` - fixes the bug/issue number `123` reported on this repository
- `Depends-On: DataDog/dd-trace-go#456` - depends on issue/PR number `456` in the `DataDog/dd-trace-go` repository

### Checks

All automated testing must pass before a PR is eligible for merging. PRs will failing automated tests will not be
reviewed in priority.

We expect PRs to include new tests for any added or significantly updated functionality; unless existing tests provide
adequate coverage for the changed surface. The CodeCov integration can help you get a sense of what the test coverage
for your change is. Reviewers may request additional tests be added before approving a change.

#### Integration Tests

There is [an `orchestrion` integration test suite in `dd-trace-go`][dd-trace-go] that validates provided integration
configurations. This test suite is executed as part of orchestrion's CI. It can be executed locally using the following
commands:

```console
$ git clone github.com:DataDog/dd-trace-go         # Clone the DataDog/dd-trace-go repository
$ cd dd-trace-go/internal/orchestrion/_integration # Move into the integration tests directory
$ go mod edit \                                    # Use the local copy of orchestrion
    -replace "github.com/DataDog/orchestrion=>${orchestrion_dir}"
$ go mod tidy                                      # Make sure go.mod & go.sum are up-to-date
$ go run github.com/DataDog/orchestrion \          # Run integration test suite with orchestrion
    go test -shuffle=on ./...
```

> See also the [Makefile](Makefile) for more details on how to run the integration tests locally.

### What to expect

We try to review new PRs within a week or so of creation. If your PR passes all automated tests and has been waiting for
a review for more than a week, feel free to comment on it to bubble it up.

PRs that have been reviewed and are left open for more than a month awaiting updates or replies by the author may be
closed due to staleness. If you want to resume working on a PR that was closed for staleness at a later point, feel free
to open a new PR.

### Code Style

All Go code must be formatted using `go fmt` so that it is in "canonical go format". YAML files must be consistently
formatted. We run `golangci-lint` and other linters as part of our automated testing suite.

> See also the [Makefile](Makefile) for more details on how to run the linters locally.

#### Local Development Commands

The project includes a Makefile with convenient targets for local development. Run `make help` to see all available targets:

<!-- markdownlint-disable MD053 MD031 -->
[embedmd]:# (tmp/make-help.txt)
```txt
Usage: make [target]

Targets:
  help                 Show this help message
  build                Build orchestrion binary to bin/orchestrion
  install              Install orchestrion to $$GOPATH/bin
  format               Format Go code and YAML files
  format/go            Format Go code only
  format/yaml          Format YAML files only (excludes testdata)
  lint                 Run all linters (Go, YAML, GitHub Actions, Makefiles)
  lint/action          Lint GitHub Actions workflows
  lint/go              Run golangci-lint on Go code
  lint/yaml            Lint YAML formatting
  lint/makefile        Lint Makefiles
  ratchet/pin          Pin GitHub Actions to commit SHAs
  ratchet/update       Update pinned GitHub Actions to latest versions
  ratchet/check        Verify all GitHub Actions are pinned
  docs                 Update embedded documentation in markdown files
  tmp/make-help.txt    Generate make help output for embedding in documentation
  test                 Run unit tests
  test-e2e             Run end-to-end tests
  test-integration     Run integration tests with dd-trace-go
```

All formatting and linting checks are enforced in CI. Run `make format` and `make lint` before submitting a PR to ensure
your changes pass automated checks.

<!-- Links -->
[new-issue]: https://github.com/DataDog/orchestrion/issues/new/choose
[conventional-commits]: https://www.conventionalcommits.org/en/v1.0.0/
[dd-trace-go]: https://github.com/DataDog/dd-trace-go/internal/orchestrion/_integration
