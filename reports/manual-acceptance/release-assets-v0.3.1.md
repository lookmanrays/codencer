# Release Assets v0.3.1 Patch Acceptance

- Branch: `fix/v0.3.1-release-assets-and-run-id-validation`
- Date: `2026-07-07`
- Verdict: GO for patch PR readiness. The existing `v0.3.0` release remains
  asset-backfill pending until the documented manual workflow dispatch is run
  after this PR is merged.

## Live Release Inspection

- `gh release view v0.3.0 --json tagName,targetCommitish,isDraft,isPrerelease,assets,url`
  showed `assets: []` for `v0.3.0`.
- Release URL: `https://github.com/lookmanrays/codencer/releases/tag/v0.3.0`
- Release target commit: `186170e9b1b2b53efa018cb37f12c10ad1f01b05`
- `gh run list --workflow "Release Please" --limit 10` showed:
  - `28816492285` failed for `chore: release 0.3.0 (#4)`.
  - `28810066852` succeeded for the earlier public self-host merge.

## Observed Root Cause

The `v0.3.0` release run did create the GitHub Release and did build both
workflow artifacts:

- `build-linux-amd64`: passed.
- `build-macos-host`: passed.
- `publish-assets`: failed.

The failed publish step was `Fail if release assets already exist`. Its log
ended with:

```text
failed to run git: fatal: not a git repository (or any of the parent directories): .git
```

The publish job did not check out the repository and called
`gh release view "$TAG_NAME"` without `--repo "$GITHUB_REPOSITORY"`. The `gh`
CLI tried to infer the repository from Git metadata, failed, and skipped the
upload steps. The release therefore remained source-archive-only.

## Implementation Summary

- Added reusable `.github/workflows/release-assets.yml`.
- Updated `.github/workflows/release-please.yml` so Release Please creates the
  release and then calls `Release Assets` when `release_created == true`.
- Removed duplicated asset build/upload jobs from `release-please.yml`.
- Removed the completed one-time `release-as: 0.3.0` bootstrap forcing from
  `release-please-config.json`.
- Documented the one-time `v0.3.0` asset backfill path.
- Added public-boundary checks for the release asset workflow shape and for
  removal of `release-as`.
- Fixed supplied run ID validation in `internal/localexec/service.go`.
- Added the daemon-side invariant in `internal/service/run_service.go` so direct
  API/MCP/relay callers cannot create phases or steps for nonexistent runs.
- Updated the public self-host verifier to create real runs before valid
  `codencer submit --run ...` checks, and to prove nonexistent supplied run IDs
  are rejected with `invalid_input`.
- Changed daemon-generated run IDs from Unix seconds to Unix nanoseconds so
  back-to-back API run creation cannot collide during release verification.
- Made Release Assets `manifest.json` generation deterministic across retries
  by deriving `built_at` from the release commit timestamp instead of the
  publish job wall clock.

## Run ID Validation

- `Submit` trims supplied run IDs.
- Empty run ID keeps the existing fresh `StartRun` behavior.
- Non-empty run ID now calls daemon `GetRun` before submitting.
- Missing supplied run IDs fail fast with `invalid_input` and do not call
  `SubmitTask`.
- Runs that belong to another project fail with:
  `run <id> belongs to project <actual>, not <expected>`.
- The daemon checks `runsRepo.Get(ctx, runID)` before phase creation, so
  `POST /api/v1/runs/missing/steps` returns 400 and leaves no orphan phase or
  step.
- The public self-host verifier now includes both sides of the contract:
  missing supplied run IDs fail, while valid supplied run IDs are created via
  `codencer run start` before submit.
- The daemon API now auto-generates run IDs with nanosecond precision. A
  regression test posts two runs without IDs and asserts both are created with
  distinct IDs.

## CI Follow-up

The first patch PR attempt failed GitHub Actions in
`make verify-public-selfhost-release` because the verifier still used
`codencer submit --run public-redaction-run` without creating
`public-redaction-run` first. That behavior is now intentionally rejected by the
new validation.

The verifier was corrected to:

- assert a nonexistent supplied run ID fails with
  `blocker: invalid_input run public-missing-run not found`;
- create a run via `codencer run start --json` before the valid supplied-run
  submit proof;
- create a separate run before the interrupt/blocker proof.

Local reproduction then exposed a second verifier-only issue: the daemon
generated default run IDs with `time.Now().Unix()`, so two back-to-back
`run start` calls could collide in the same second. The daemon now uses
`time.Now().UTC().UnixNano()` for generated run IDs, and the regression is
covered in `internal/app/api_test.go`.

A later macOS CI rerun exposed a flaky cleanup assertion in
`TestRunService_RetryStepReconcilesRunBackToRunning`: the test was validating
immediate persisted retry state, then failing if best-effort abort cleanup raced
with asynchronous workspace provisioning. The cleanup is now non-fatal for that
state-reconciliation test, and the targeted retry test passed 20 consecutive
local runs.

## Manifest Retry Determinism

Review found that `manifest.json` used `datetime.now(...)` for `built_at`. If a
publish retry ran after binary assets and `manifest.json` had already uploaded,
the regenerated manifest differed even when the binary archives were identical,
so `replace_existing=false` could fail before the intended skip-identical path.

The Release Assets workflow now resolves `built_at` during preflight from the
release commit timestamp and passes it to the publish job. For the same tag,
release SHA, and binary archive inputs, generated `manifest.json` is stable
across retries.

## v0.3.0 Backfill Procedure

After this PR is merged, run:

```text
Actions -> Release Assets -> Run workflow
tag_name: v0.3.0
ref: v0.3.0
replace_existing: false
```

Expected assets:

- `codencer_v0.3.0_linux_amd64.tar.gz`
- `codencer_v0.3.0_darwin_arm64.tar.gz`
- `codencer_v0.3.0_darwin_amd64.tar.gz`
- `checksums.txt`
- `manifest.json`

This backfill has not been run from Codex.

## Verification

- `gh release view v0.3.0 --json tagName,targetCommitish,isDraft,isPrerelease,assets,url` - passed; assets were empty.
- `gh run list --workflow "Release Please" --limit 10` - passed.
- `gh run view 28816492285 --json jobs` - passed.
- `gh run view 28816492285 --log-failed` - passed.
- `python3 -m json.tool release-please-config.json` - passed.
- Ruby YAML parse for `.github/workflows/release-please.yml`, `.github/workflows/release-assets.yml`, and `.github/workflows/semantic-pr-title.yml` - passed.
- `actionlint .github/workflows/release-please.yml .github/workflows/release-assets.yml .github/workflows/semantic-pr-title.yml` - skipped because `actionlint` is not installed locally.
- `python3 scripts/check_public_boundary.py` - passed with deterministic
  manifest guard.
- `go test ./internal/service -run TestRunService_RetryStepReconcilesRunBackToRunning -count=20` - passed after macOS cleanup stabilization.
- `go test ./internal/service` - passed.
- `gofmt -w internal/localexec/service.go internal/localexec/service_test.go internal/service/run_service.go internal/service/run_service_test.go internal/app/api_test.go` - passed.
- `go test ./internal/localexec ./internal/service ./internal/app ./cmd/codencer` - passed.
- `go test ./...` - passed.
- `make build && ./scripts/verify_public_selfhost_release.sh` - passed after
  the CI follow-up verifier fix and daemon run-ID collision fix.
- `make verify-public-selfhost-release TARGETS=host REQUIRE_TARGETS=host` -
  passed.
- `make verify-public-release` - passed.
- `git diff --check` - passed.
- `git diff --cached --check` - passed.
