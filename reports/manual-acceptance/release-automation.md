# Release Automation Acceptance

- Branch: `next-phase`
- Date: `2026-07-06`
- Verdict: GO for repository changes; GitHub release publication remains pending
  until this branch is merged to `main`.

## Implementation Summary

- Added a main-only `Release Please` workflow.
- Added Release Please manifest configuration for a root package using the
  `simple` strategy and explicit v-tagging.
- Added root `version.txt` with `0.2.0` so the first Release Please run has the
  expected `simple` strategy version file.
- Bootstrapped the first public automated release with one-time
  `release-as: 0.3.0`.
- Added GitHub Release asset build/upload in the same workflow that creates the
  release, avoiding a separate `release.created` workflow.
- Added PR title validation for Conventional Commit titles.
- Added release automation docs and updated the release checklist so local
  snapshot builds are emergency/debug only.

## Release Behavior

- Pushes to `main` run Release Please.
- Normal work lands through Conventional Commit PR titles.
- Release Please opens or updates a Release PR.
- Merging the Release PR creates the GitHub Release.
- If `release_created=true`, the same workflow builds:
  - `linux/amd64` on `ubuntu-latest`;
  - `darwin/arm64` and `darwin/amd64` on `macos-latest`.
- The publish job generates `checksums.txt` and a truthful `manifest.json`.
- Release asset upload uses `gh release upload` without `--clobber`; existing
  assets fail the workflow.

## Bootstrap And Future Versions

- The first automated public release is forced to `v0.3.0`.
- `version.txt` starts at `0.2.0`, is owned by Release Please, and is used only
  for release automation.
- The first Release PR should update `CHANGELOG.md`, `version.txt`, and
  `.release-please-manifest.json`.
- After `v0.3.0` is released, remove `release-as: 0.3.0`.
- Future releases are computed from Conventional Commits:
  - `fix:` => patch;
  - `feat:` => minor;
  - `feat!:` or `BREAKING CHANGE:` => major.

## Boundary Notes

- No real tag was created.
- No GitHub Release was published.
- No production release was run.
- No local release artifacts are committed or uploaded.
- Existing public testability workflows remain verification-only.

## Verification

- `python3 -m json.tool release-please-config.json` - passed.
- `python3 -m json.tool .release-please-manifest.json` - passed.
- `test "$(cat version.txt)" = "0.2.0"` - passed.
- Ruby YAML parse for `.github/workflows/release-please.yml` - passed.
- Ruby YAML parse for `.github/workflows/semantic-pr-title.yml` - passed.
- PR title regex sanity check - passed.
- `actionlint .github/workflows/release-please.yml .github/workflows/semantic-pr-title.yml` - skipped because `actionlint` is not installed locally.
- `go test ./...` - passed.
- `make verify-public-release` - passed.
- `git diff --check` - passed.
- `git diff --cached --check` - passed.
