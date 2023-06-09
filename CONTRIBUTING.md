### Contributing

Thanks for your interest in contributing! This is an open source project, so we appreciate community contributions.

Pull requests for bug fixes are welcome, but before submitting new features or changes to current functionalities open an issue and discuss your ideas or propose the changes you wish to make. After a resolution is reached a PR can be submitted for review. PRs created before a decision has been reached may be closed.

For commit messages, try to use the same conventions as most Go projects, for example:

```
cmd/orchestrion: add -rm flag to remove instrumentation

This commit adds the 'rm' flag to the orchestrion command, which causes
orchestrion to remove any instrumentation from a package.
```

Please apply the same logic for Pull Requests and Issues: start with the package name, followed by a colon and a description of the change, just like the official Go language.

All new code is expected to be covered by tests.
