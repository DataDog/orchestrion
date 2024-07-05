# Release process

## Steps

1. Determine the new release's version number
   - Follow [Semantic Versioning 2.0](https://semver.org/spec/v2.0.0.html) semantics
      + Be mindful of the `v0.x.y` semantics!
   - The placeholder `vX.Y.Z` is used to refer to the tag name including this version number in all
     steps below
1. Check out the repository on the correct commit, which is most likely `origin/main`
   ```console
   $ git fetch
   $ git checkout origin/main -b ${USER}/release/vX.Y.Z
   ```
1. Edit [`internal/version/version.go`](/internal/version/version.go) to set the `Tag` constant to
   the new `vX.Y.Z` version
1. Commit the resulting changes
   ```console
   $ git commit -m "release: vX.Y.Z" internal/version/version.go
   ```
1. Open a pull request
   ```console
   $ gh pr create --web
   ```
1. Get the PR reviewed by a colleague, ensure all CI passes including the _Release_ validations
1. Get the PR merged to `main` via the merge queue
1. Once merged, a draft release will automatically be created on GitHub
   - Locate it on the [releases](https://github.com/DataDog/orchestrion/releases) page
   - Review the release notes, and edit them if necessary:
      + Remove `chore:` entries
      + Fix any typos you notice
1. Once validated, publish the release on GitHub
   - This automatically creates the release tag, so you're done!
