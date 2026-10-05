# Release process

## Steps

1. Determine the new release's version number
   - Follow [Semantic Versioning 2.0](https://semver.org/spec/v2.0.0.html) semantics
      - Be mindful of the `v0.x.y` semantics!
   - The placeholder `vX.Y.Z` is used to refer to the tag name including this version number in all
     steps below
1. Check out the repository on the correct commit, which is most likely `origin/main`

   ```console
   git fetch
   git checkout origin/main -b ${USER}/release/vX.Y.Z
   ```

1. Edit [`internal/version/version.go`](/internal/version/version.go) to set the `Tag` constant to
   the new `vX.Y.Z` version
1. Commit the resulting changes

   ```console
   git commit -m "release: vX.Y.Z" internal/version/version.go
   ```

1. Open a pull request

   ```console
   gh pr create --web
   ```

1. Get the PR reviewed by a colleague, ensure all CI passes including the _Release_ validations
1. Get the PR merged to `main` via the merge queue
1. Once merged, a draft release will automatically be created on GitHub (see [.github/release.yml](.github/release.yml) for more details)
   - Locate it on the [releases](https://github.com/DataDog/orchestrion/releases) page
   - Review the release notes, and edit them if necessary:
      - Remove `chore:` entries
      - Fix any typos you notice
1. Once validated, publish the release on GitHub
   - Publishing creates the root release tag. Confirm that the `Tag Release` job also succeeds so
     nested modules are tagged at the same commit. See the recovery instructions below if it fails.

## Release tag recovery

Publishing a GitHub release creates its root tag. The `Tag Release` job then creates annotated tags for
publishable nested modules at the same commit, using a repository-scoped `dd-octo-sts` token.
The trust policies in `.github/chainguard/self.github.release.*.sts.yaml` restrict that token to the
release workflow: published-release events on version tags, or manual recovery from `main`.

If nested-module tagging fails, repair the published release using the reviewed workflow on `main`:

```console
gh workflow run release.yml --repo DataDog/orchestrion --ref main -f tag=v1.13.1
```

Replace `v1.13.1` with the published root release tag. The job rejects drafts, skips existing tags that
resolve to the correct commit, and fails on conflicting tags without moving them. It reads module
metadata from the root tag's commit, not from the current `main` checkout. After the job finishes,
verify the nested tags and check that the Go module proxy resolves them; cached misses can delay
proxy availability.

A release event uses the workflow revision associated with that release. Merging an automation fix
into `main` does not update an already-cut release's workflow. Use the manual recovery command for
such releases rather than moving the published root tag.

The default `GITHUB_TOKEN` cannot create tags under the organization tag-protection rules. If token
exchange succeeds but tag creation still fails with `GH013`, ask SDLC Security to verify the STS
App's authorization. Do not disable tag protection or fall back to a stored personal token.
