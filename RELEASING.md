# Release process

## Overview

The release process consists of creating a release branch, merging fixes to `main` **and** to the release branch, and releasing release candidates as things progress. Once a release candidate is stable, a final version can be released.

## Steps

### Create the release branch and the first release candidate

1. Checkout the repository on the correct branch and changeset (`main`).
2. Create a new release branch: `git checkout -b vX.Y`.
3. Add a tag for the first release candidate: `git tag vX.Y.Z-rc.1`.
4. Push the branch and tag.

   ```console
   $ git push origin vX.Y
   $ git push origin vX.Y.Z-rc.1
   ```
5. Bump the tag in the `internal/version/version.go` file in the main branch to the next minor pre-release version using the `-dev` pre-release suffix.

### Create a release candidate after a bug fix

**Note:** The fix must be merged to `main` and backported the release branch.

1. Update the release branch `vX.Y` locally by pulling the bug fix merged upstream (`git fetch`, `git pull`)
2. Modify the version string in `internal/version/version.go` to the release candidate version.
3. Add a tag for the new release candidate: `git tag vX.Y.Z-rc.W`.
4. Push the branch and tag.

   ```console
   $ git push origin vX.Y
   $ git push origin vX.Y.Z-rc.W
   ```

### Release the final version

1. Update the release branch `vX.Y` locally by pulling any bug fixes merged upstream (`git fetch`, `git pull`)
2. Modify the version string in `internal/version/version.go` to the final version.
3. Add a final release tag: `git tag vX.Y.Z`.
4. Push the branch and tag.

   ```console
   $ git push origin vX.Y
   $ git push origin vX.Y.Z-rc.W
   ```

5. Create a [GitHub release](https://github.com/DataDog/orchestrion/releases/new). 
    - Choose the version tag `vX.Y.Z`
    - Set the release title to `vX.Y.Z`
    - Click on `Generate release notes` for automatic release notes generation
    - Click on `Publish release`
