# Versions and releases

**v0.1.0 is the planned first release, not an existing published release.**
Maintainers choose versions manually. There is no release per PR and no version
inference from commit messages.

## Compatibility policy

ExpLedger follows [Semantic Versioning](https://semver.org/). Within SemVer's
initial development phase, this project uses these conventions:

- During `0.x`, compatible fixes and small internal improvements increment the
  patch version (for example, `0.1.0` to `0.1.1`).
- New functionality or breaking changes increment the minor version (for
  example, `0.1.1` to `0.2.0`). Breaking changes require clear migration notes.
- `1.0.0` marks a stable public interface. After that, incompatible public
  changes increment major, compatible features minor, and compatible fixes patch.

The public interface includes commands, flags, configuration, experiment
records, and documented execution behavior. Internal Go packages are not part
of that interface.

Application versions and record schema versions are independent. Records keep
`schema: expledger/v1` until an incompatible stored-format change requires a new
schema revision. Application releases do not automatically change record formats.

Published tags and assets stay unchanged. Correct a published release with a new
patch release; never move its tag or replace its archives. Enable GitHub's
immutable releases and protect release tags in repository settings before the
first release to enforce this policy outside CI as well.

## Maintainer procedure

1. Merge reviewed changes to `main` and select the exact merged commit to release.
2. Choose an unused version manually. Write short release notes in a local file,
   including migration instructions for any breaking changes. No `VERSION` file
   or source version edit is needed.
3. After final approval, create an annotated tag on that commit and push only it.
   For the planned first release, the commands would be:

   ```sh
   git fetch origin main
   git tag -a v0.1.0 <chosen-commit> -F /path/to/release-notes.md
   git push origin refs/tags/v0.1.0
   ```

4. The tag workflow checks a stable `vMAJOR.MINOR.PATCH` name, annotation,
   ancestry on `main`, and absence of an existing GitHub Release. It tests and
   vets that exact tagged commit, builds the four archives with pinned
   GoReleaser v2.18.2, verifies their licenses and SHA-256 checksums, and runs a
   native Linux version smoke test outside a project. Only then does it publish
   a new GitHub Release using the tag annotation as the notes.
5. Inspect the completed workflow and Release assets. A failed run that has not
   created a Release can be retried without moving the tag. If a Release exists,
   use a new patch version for corrections; CI refuses to reuse it.
   A failed asset upload can leave a draft Release, which also blocks a retry;
   inspect the failure and draft before choosing the next version.

Only tag pushes trigger publishing. PRs exercise the same packaging configuration
as a non-publishing development snapshot with read-only permissions. The release
job alone receives `contents: write`; there is no `pull_request_target` event.
An annotated tag is the publication authorization, so protect tag creation.

## Local packaging check

Install the pinned [GoReleaser](https://goreleaser.com/getting-started/quick-start/)
version, then from a checkout run:

```sh
goreleaser check
goreleaser release --snapshot --clean
scripts/check-release.sh devel
```

GoReleaser must be v2.18.2 to match CI. Output is ignored under `release-dist/`.
The check script inspects all four `.tar.gz` files, compares their licenses,
checks SHA-256 sums, and executes only the host platform's binary. Cross-compiling
other targets does not prove that they execute successfully on those platforms.

See [installation](install.md) for archive and exact-version Go installation.
