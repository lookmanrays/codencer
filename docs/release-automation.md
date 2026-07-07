# Release Automation

Codencer releases are managed from `main` by Release Please plus the reusable
Release Assets workflow. Normal public releases do not require local artifact
upload or manual version input.

## Main-Only Flow

1. Land work through pull requests with Conventional Commit titles.
2. Merge PRs to `main`.
3. Release Please opens or updates a Release PR.
4. Review the changelog and version in the Release PR.
5. Merge the Release PR when the release is ready.
6. Release Please creates the GitHub Release.
7. The Release Please workflow calls `.github/workflows/release-assets.yml` to
   build release artifacts, generate `checksums.txt` and `manifest.json`, and
   upload them.
8. Verify the GitHub Release page has the Linux artifact, one macOS host
   artifact, checksums, and manifest.

The asset build/upload is a reusable workflow called by Release Please because
GitHub resources created with `GITHUB_TOKEN` do not reliably trigger separate
`release.created` workflows.
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

The first automated public release bootstrap is complete. `v0.3.0` has already
been released, and `release-please-config.json` must not keep
`release-as: 0.3.0`. Future versions are computed from Conventional Commits.
The patch that introduced the release asset workflow is expected to produce a
`v0.3.1` Release PR because it is a `fix:` change.

`version.txt` and `.release-please-manifest.json` are then advanced by Release
Please PRs only.

## v0.3.0 Asset Backfill

The `v0.3.0` GitHub Release was created before the reusable Release Assets
workflow existed and may have only GitHub source archive links. Source archives
are not installable Codencer product artifacts.

After the release asset workflow PR is merged to `main`, backfill `v0.3.0` from
GitHub:

1. Open `Actions -> Release Assets -> Run workflow`.
2. Set `tag_name` to `v0.3.0`.
3. Set `ref` to `v0.3.0`.
4. Set `replace_existing` to `false`.
5. Run the workflow.
6. Verify the `v0.3.0` release now has:
   - `codencer_v0.3.0_linux_amd64.tar.gz`
   - `codencer_v0.3.0_darwin_<host_arch>.tar.gz`
   - `checksums.txt`
   - `manifest.json`

Do not upload local artifacts manually for this backfill unless a separate
emergency release procedure explicitly approves it.

## Release Assets

The Release Assets workflow builds and uploads:

- `codencer_${TAG_NAME}_linux_amd64.tar.gz`
- `codencer_${TAG_NAME}_darwin_<host_arch>.tar.gz`
- `checksums.txt`
- `manifest.json`

The macOS artifact is a host build from `macos-latest`. Do not claim both
`darwin/amd64` and `darwin/arm64` unless both are actually built.

Installable public Codencer releases are the attached `codencer_*.tar.gz`
binary archives. The GitHub-generated source ZIP/TAR links are source snapshots
only; they are not installable Codencer binary release artifacts. Operators
download the archive for their platform, verify it against `checksums.txt`,
extract it, and use the binaries from the archive `bin/` directory.

`manifest.json` records:

- version and tag name;
- release SHA;
- deterministic `built_at` timestamp derived from the release commit timestamp;
- asset filename, SHA256, OS, architecture, and runner;
- a note that artifacts were built by GitHub Actions.

The upload step does not use `--clobber`. With `replace_existing=false`, an
existing identical asset is skipped and an existing differing asset fails the
workflow. `manifest.json` is deterministic for the same tag, release SHA, and
binary archive inputs so a retry after a partial publish can follow the same
skip-identical path. With `replace_existing=true`, only the exact known
Codencer target asset names are deleted and replaced; GitHub source archives are
never deleted.

Release Please creates non-draft releases today. If asset upload or verification
fails, the Release Assets job fails loudly so the release cannot be mistaken for
a complete binary release.

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
