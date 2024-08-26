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

### What to expect

We try to review new PRs within a week or so of creation. If your PR passes all automated tests and has been waiting for
a review for more than a week, feel free to comment on it to bubble it up.

PRs that have been reviewed and are left open for more than a month awaiting updates or replies by the author may be
closed due to staleness. If you want to resume working on a PR that was closed for staleness at a later point, feel free
to open a new PR.

### Code Style

All Go code must be formatted using `go fmt` so that it is in "canonical go format". We run `golangci-lint` as part of
our automated testing suite.

<!-- Links -->
[new-issue]: https://github.com/DataDog/orchestrion/issues/new/choose
[conventional-commits]: https://www.conventionalcommits.org/en/v1.0.0/
