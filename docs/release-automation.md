# Release Automation

Codencer releases are managed from `main` by Release Please. Normal public
releases do not require local artifact upload or manual version input.

## Main-Only Flow

1. Land work through pull requests with Conventional Commit titles.
2. Merge PRs to `main`.
3. Release Please opens or updates a Release PR.
4. Review the changelog and version in the Release PR.
5. Merge the Release PR when the release is ready.
6. The same workflow creates the GitHub Release, builds release artifacts,
   generates `checksums.txt` and `manifest.json`, and uploads them.
7. Verify the GitHub Release page has the Linux artifact, one macOS host
   artifact, checksums, and manifest.

The asset build/upload runs in the same workflow as Release Please because
GitHub resources created with `GITHUB_TOKEN` do not trigger separate workflows.
If `RELEASE_PLEASE_TOKEN` is configured, the workflow uses it for Release
Please so Release PR checks can trigger normally. Otherwise it uses
`GITHUB_TOKEN`.

## Versioning

Release Please reads Conventional Commit messages on `main`:

- `fix:` creates a patch release.
- `feat:` creates a minor release.
- `feat!:` or `BREAKING CHANGE:` creates a major release.

The Release PR is the human gate. Merging the Release PR creates the tag and
GitHub Release.

`version.txt` is owned by Release Please and is used only as release automation
state for the `simple` strategy. It is not runtime product configuration and
should not be edited manually outside release automation bootstrap or cleanup
work.

## First Public Release Bootstrap

The first automated public release is intentionally forced to `v0.3.0` with
`release-as: 0.3.0` in `release-please-config.json`. This avoids deriving the
first public version from older non-conventional branch history.

The repository starts with `version.txt` and `.release-please-manifest.json` at
`0.2.0`. The first Release PR should update `CHANGELOG.md`, `version.txt`, and
`.release-please-manifest.json` to `0.3.0`.

After the `v0.3.0` Release PR is merged, remove the one-time `release-as:
0.3.0` setting in a follow-up PR. Future versions should then be computed from
Conventional Commits only.

## Release Assets

The Release Please workflow builds and uploads:

- `codencer_${TAG_NAME}_linux_amd64.tar.gz`
- `codencer_${TAG_NAME}_darwin_<host_arch>.tar.gz`
- `checksums.txt`
- `manifest.json`

The macOS artifact is a host build from `macos-latest`. Do not claim both
`darwin/amd64` and `darwin/arm64` unless both are actually built.

`manifest.json` records:

- version and tag name;
- release SHA;
- build timestamp;
- asset filename, SHA256, OS, architecture, and runner;
- a note that artifacts were built by GitHub Actions.

The upload step does not use `--clobber`; existing assets fail the workflow.

## Local Snapshot Use

`make release-snapshot` remains the implementation behind automated packaging.
Running it locally is allowed for emergency/debug verification only. Local
archives are not the normal public release upload path.

Useful local checks:

```bash
make release-snapshot VERSION=v0.3.0-local-debug TARGETS=host REQUIRE_TARGETS=host
make verify-release-artifact-selfhost VERSION=v0.3.0-local-debug TARGETS=host REQUIRE_TARGETS=host
make verify-public-release
```

Do not upload local artifacts to a public release unless a documented emergency
release procedure explicitly approves it.
