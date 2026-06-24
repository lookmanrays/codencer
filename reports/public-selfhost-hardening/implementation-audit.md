# Public Self-host Hardening Implementation Audit

Audit date: 2026-06-24

Branch: `next-phase`

Current release verdict: `NO-GO`

## Source Material

The active goal requires an attached specification package with named files:

- `docs/specs/00-public-selfhost-release-gate.md`
- `docs/specs/01-local-first-source-of-truth.md`
- `docs/specs/02-execution-lifecycle.md`
- `docs/specs/03-human-interrupts-and-permissions.md`
- `docs/specs/04-cli-commands-and-control-plane.md`
- `docs/specs/05-executor-adapters-and-client-proofs.md`
- `docs/specs/06-run-history-audit-and-console.md`
- `docs/specs/07-public-vs-cloud-boundary.md`
- `docs/specs/99-research-anchors.md`
- `docs/acceptance/public-selfhost-release-gate.yaml`

I could not find attachment files containing those exact filenames or exact
document bodies in the local Codex attachment cache. I did read the active goal
text and the prior public self-host RC attachments. The spec files added to the
repository are therefore derived from the available goal/spec text. The
requirement to copy an exact provided spec package remains `unclear` because
the exact package was not available in the current attachment cache.

## Summary

| Area | Status | Notes |
| --- | --- | --- |
| Spec files present | Partially implemented | Files now exist, but exact-source fidelity is unclear. |
| Acceptance YAML present | Implemented | `docs/acceptance/public-selfhost-release-gate.yaml` exists. |
| Local-first source of truth | Partially implemented | Local daemon/CLI exists; default outputs still expose local paths/daemon URLs in some JSON structures. |
| Explicit sync/publish | Partially implemented | `codencer sync status/preview/publish` now provides metadata-only preview and structured blockers; Gateway ingest remains unimplemented. |
| Local CLI submit UX | Partially implemented | `codencer submit` exists and is local-first; default human output redacts local paths, but progress UX remains narrow. |
| Async run lifecycle | Partially implemented | Local `run start/list/get/status/events/report/cancel/resume` exists; `resume` is a structured unsupported blocker until daemon HTTP resume exists. |
| Human interrupt lifecycle | Partially implemented | Low-level gates/blockers exist; no complete planning/question/permission/OS-action resume/cancel lifecycle. |
| Real executor proofs | Partially implemented | Codex real gate exists; Claude Code and Antigravity release-gate proofs are not proven. |
| Run history/audit/console | Partially implemented | Gateway-observed run history/audit and `scope` exist; pagination, filtering breadth, and grouping remain incomplete. |
| Redaction | Partially implemented | Gateway sanitization exists; local CLI and some reports can still expose local paths/daemon URLs. |
| Public/private boundary | Partially implemented | Docs/checks exist; public repo still contains cloud-control-plane packages that need boundary review against the new specs. |
| Public RC verifier | Partially implemented | `make verify-public-selfhost-rc` exists; it still supports `PARTIAL` and does not require Claude/Antigravity proof. |

## Requirement Audit

### 00 - Public Self-host Release Gate

| Requirement | Status | Evidence |
| --- | --- | --- |
| Final verdicts only `GO` or `NO-GO` | Missing | `scripts/verify_public_selfhost_rc.sh` still emits `PARTIAL` when real executor is not configured. |
| Fake/simulation cannot satisfy GO | Partially implemented | Codex real gate rejects simulation in `web/gateway-console/tests/live/verify-live.mjs`; fake-only overall gate still can be reported as `PARTIAL`, which is now forbidden. |
| Artifact-backed verifier | Implemented | `make verify-public-selfhost-rc` builds/unpacks artifacts through `scripts/verify_public_selfhost_rc.sh`. |
| Codex, Claude Code, and Antigravity real proofs | Missing | Codex path exists; no equivalent required release proof for Claude Code or Antigravity found. |
| Missing reports/audit/path leaks fail gate | Partially implemented | Existing Gateway live verifier checks report/audit/leaks; local sync preview redaction is tested; pagination/grouping checks remain incomplete. |
| Machine-readable and human report | Implemented | `reports/public-selfhost-rc/<timestamp>/summary.json` and `.md` are produced by the RC script. |

### 01 - Local-first Source of Truth

| Requirement | Status | Evidence |
| --- | --- | --- |
| Manual local CLI runs stay local by default | Partially implemented | `codencer submit` calls `internal/localexec.Service.Submit`; no Gateway dependency by default. |
| Gateway is control plane/index/sync target, not global source of truth | Partially implemented | Gateway records Gateway-observed run history and local sync preview reports `scope=local`; actual sync ingest is not implemented. |
| Raw logs/artifacts not uploaded by default | Partially implemented | Gateway sanitizes report JSON; `codencer sync` is metadata-only and blocks raw artifact/log upload. Local reports can still contain local refs on disk. |
| Explicit sync/publish behavior | Partially implemented | `codencer sync status/preview/publish` exists; publish returns structured confirmation/unsupported blockers rather than uploading implicitly. |
| Default output does not leak local paths | Partially implemented | Gateway sanitization exists; `localexec.ProjectSummary` includes `repo_root` and `daemon_url`, and run plan reports include `report_path`. |

### 02 - Execution Lifecycle

| Requirement | Status | Evidence |
| --- | --- | --- |
| Submit/status/events/report/cancel/resume lifecycle | Partially implemented | Local `run start/list/get/status` exists; Gateway `/runs`, `/runs/{id}`, `/runs/{id}/events`, and report endpoint exist. No complete local/Gateway cancel/resume/report/events CLI contract. |
| Long-running tasks not dependent on one blocking request | Partially implemented | Local submit can run without `--wait`; Gateway MCP still exposes `codencer.submit_project_task_and_wait` and Gateway Console submits through a blocking Relay call. |
| `get_run_report` for simple and manifest runs | Implemented for covered Gateway paths | Gateway tests cover submit/get report and manifest report paths. |
| Run state transitions include waiting/canceled/resumed | Partially implemented | Domain has states/gates in daemon tests; Gateway history does not expose the full lifecycle contract. |

### 03 - Human Interrupts and Permissions

| Requirement | Status | Evidence |
| --- | --- | --- |
| Planning approval required | Partially implemented | Low-level gates/manual approval exist in daemon/app tests. Not exposed as release-level CLI/MCP/UI lifecycle. |
| Clarifying questions | Partially implemented | `localexec.Blocker` has `Questions`; no first-class resume/answer flow found. |
| Permission requests | Partially implemented | Dangerous executor confirmation exists in Gateway Console; no generalized permission-request lifecycle. |
| OS/system human action required | Missing | No first-class OS/system action interrupt model found. |
| Resume/cancel/audit interrupt lifecycle | Missing | Abort/cancel exists in lower-level routes; no complete public interrupt lifecycle. |

### 04 - CLI Commands and Control Plane

| Requirement | Status | Evidence |
| --- | --- | --- |
| `codencer submit` local-first command | Partially implemented | Exists in `cmd/codencer/main.go`; needs better default progress/result/redaction behavior. |
| `codencer run status` | Implemented | `run status` exists. |
| `codencer run events` | Missing | No `run events` subcommand found. |
| `codencer run report` | Missing | `run get` exists; no explicit report command found. |
| `codencer run cancel` | Missing | No local CLI cancel subcommand found. |
| `codencer run resume` | Missing | No local CLI resume subcommand found. |
| `codencer executor list/scan/test/default` | Implemented | Implemented in `cmd/codencer/main.go`. |
| `codencer sync` or publish equivalent | Partially implemented | `codencer sync status/preview/publish` exists with metadata-only preview and no raw upload. |
| Public defaults are local/self-host | Partially implemented | Config/default docs and scripts exist; needs re-check against new specs. |

### 05 - Executor Adapters and Client Proofs

| Requirement | Status | Evidence |
| --- | --- | --- |
| Codex real executor proof | Implemented | `verify-public-selfhost-rc` supports `CODENCER_E2E_REAL_EXECUTOR=codex`; live verifier rejects simulation. |
| Claude Code real executor proof | Missing | Claude adapter tests exist, but no required artifact-backed public release proof. |
| Antigravity real executor proof | Missing | Adapter/broker code exists, but no required artifact-backed public release proof. |
| Simulation guard | Partially implemented | Codex/Gateway live verifier has strong simulation checks; not generalized to all required executors. |
| Fake never satisfies GO | Missing under new verdict rules | Existing RC verifier can still produce `PARTIAL`, not `NO-GO`; fake-only path is not fully disallowed as a release result. |

### 06 - Run History, Audit, and Console

| Requirement | Status | Evidence |
| --- | --- | --- |
| Compact run history | Implemented | `/api/gateway/v1/runs` and `/console/runs` exist. |
| Run detail | Implemented | `/api/gateway/v1/runs/{id}` and `/console/runs/[id]` exist. |
| Audit lifecycle events | Implemented for Gateway-observed runs | Gateway records task/route/relay/connector/executor/start/terminal/report events. |
| Pagination | Partially implemented | `limit` exists; no offset/cursor/next token. |
| Filters | Partially implemented | Runs support project/status; audit has no visible filter API beyond limit. |
| Grouped audit | Partially implemented | Events link to run detail; no grouped audit API/view found. |
| Local/synced/Gateway-submitted scopes | Partially implemented | Gateway run records expose `scope=gateway_submitted`; sync preview reports local run metadata as `scope=local`. No Gateway synced ingest yet. |
| Execution mode visible | Implemented for current UI | Gateway Console shows `Real executor`, `Simulation`, or `Unknown`. |

### 07 - Public vs Cloud Boundary

| Requirement | Status | Evidence |
| --- | --- | --- |
| Public/self-host defaults | Partially implemented | Public release checks exist. Needs re-audit after spec addition. |
| Private managed service code absent | Partially implemented | Public repo still has `internal/cloud` and `cmd/codencer-cloud*` packages; existing boundary docs classify them, but new spec may require stricter handling. |
| Hosted proof not claimed unless run | Partially implemented | Docs were previously cleaned; must re-run stale/boundary checks after new specs. |

## Immediate Blocking Items for GO

The release remains `NO-GO` until at least these are resolved:

1. `verify-public-selfhost-rc` must emit only `GO` or `NO-GO`; `PARTIAL` is forbidden.
2. Claude Code real executor proof must be implemented or the final verdict must remain `NO-GO`.
3. Antigravity real executor proof must be implemented or the final verdict must remain `NO-GO`.
4. Async lifecycle must include submit/status/events/report/cancel/resume behavior or explicit structured capability blockers where unsupported.
5. Human interrupt lifecycle must be first-class across CLI/MCP/UI/Gateway or explicitly proven with structured blockers and audit.
6. Local CLI output must be redacted by default, especially `repo_root`, `daemon_url`, `report_path`, `logs_ref`, and artifact paths.
7. Run history/audit must add required pagination, broader filters, and grouping.
8. Gateway metadata ingest for explicit sync/publish remains unimplemented; current CLI preview/publish behavior is safe but not a completed sync transport.
9. The final hardening report must end with exactly `Verdict: GO` or `Verdict: NO-GO`.
