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
| Local-first source of truth | Partially implemented | Local daemon/CLI exists; default project/status/run/submit human output is redacted, while explicit JSON/debug outputs still carry local state for operator tooling. |
| Explicit sync/publish | Partially implemented | `codencer sync status/preview/publish` now provides metadata-only preview; confirmed publish ingests sanitized metadata into Gateway run history. Raw logs/artifacts remain blocked. |
| Local CLI submit UX | Partially implemented | `codencer submit` exists and is local-first; default human output redacts local paths, but progress UX remains narrow. |
| Async run lifecycle | Partially implemented | Local `run start/list/get/status/events/report/cancel/resume` exists; Gateway MCP now exposes async start/submit/list/status/report/events and structured cancel/resume capability blockers. Console submit still uses a blocking report path for immediate result display. |
| Human interrupt lifecycle | Partially implemented | Local reports/events now expose first-class `human_interrupts`, Gateway blocker outcomes emit `human_interrupt_created` audit events, and Antigravity unsafe permission waits now fail fast as manual-attention results; complete answer/resume UI/MCP lifecycle remains incomplete. |
| Real executor proofs | Partially implemented | Codex has current artifact-backed proof; earlier Claude Code proof exists; Antigravity remains unproven and now fails early when the provided LS workspace does not match the isolated verifier repo. |
| Run history/audit/console | Partially implemented | Gateway-observed run history/audit now includes scope, limit/offset pagination, server-side filters, and grouped lifecycle summaries; synced/local ingest transport remains incomplete. |
| Redaction | Partially implemented | Gateway/sync sanitization exists and default local human CLI output is tested for path/daemon URL redaction; full cross-surface redaction proof is still incomplete. |
| Public/private boundary | Partially implemented | Docs/checks exist; public repo still contains cloud-control-plane packages that need boundary review against the new specs. |
| Public RC verifier | Partially implemented | `make verify-public-selfhost-rc` emits only `GO`/`NO-GO`, requires configured real-proof coverage, and reports `NO-GO` when required proofs are missing; Antigravity remains unproven. |

## Requirement Audit

### 00 - Public Self-host Release Gate

| Requirement | Status | Evidence |
| --- | --- | --- |
| Final verdicts only `GO` or `NO-GO` | Implemented | `scripts/verify_public_selfhost_rc.sh` emits `GO` or `NO-GO`; no `PARTIAL` branch remains. |
| Fake/simulation cannot satisfy GO | Implemented for current verifier | Real executor gates reject simulation text/metadata and missing required real proofs force `NO-GO`; Codex and Claude real gates passed with simulation disabled. |
| Artifact-backed verifier | Implemented | `make verify-public-selfhost-rc` builds/unpacks artifacts through `scripts/verify_public_selfhost_rc.sh`. |
| Codex, Claude Code, and Antigravity real proofs | Partially implemented | Codex passed current artifact-backed scoped proof in `reports/public-selfhost-rc/20260624T120012Z`; Codex and Claude Code passed earlier artifact-backed real gates in `reports/public-selfhost-rc/20260624T105654Z`; Antigravity remains missing. |
| Missing reports/audit/path leaks fail gate | Partially implemented | Existing Gateway live verifier checks report/audit/leaks and now asserts run/audit pagination plus grouped audit arrays; human interrupt audit is covered for blocker outcomes, but multi-executor proof coverage remains incomplete. |
| Machine-readable and human report | Implemented | `reports/public-selfhost-rc/<timestamp>/summary.json` and `.md` are produced by the RC script. |

### 01 - Local-first Source of Truth

| Requirement | Status | Evidence |
| --- | --- | --- |
| Manual local CLI runs stay local by default | Partially implemented | `codencer submit` calls `internal/localexec.Service.Submit`; no Gateway dependency by default. |
| Gateway is control plane/index/sync target, not global source of truth | Partially implemented | Gateway records Gateway-observed run history; local sync preview reports `scope=local`; confirmed sync publish creates sanitized `scope=synced` history records. |
| Raw logs/artifacts not uploaded by default | Partially implemented | Gateway sanitizes report JSON; `codencer sync` is metadata-only and blocks raw artifact/log upload. Local reports can still contain local refs on disk. |
| Explicit sync/publish behavior | Partially implemented | `codencer sync status/preview/publish` exists; publish requires `--confirm`, requires login, blocks raw artifact/log requests, and sends only sanitized metadata. |
| Default output does not leak local paths | Partially implemented | Default human output for project/status/submit/run events/run report is redacted and tested; explicit `--json` reports still include local `repo_root`, `daemon_url`, and `report_path` for operator tooling. |

### 02 - Execution Lifecycle

| Requirement | Status | Evidence |
| --- | --- | --- |
| Submit/status/events/report/cancel/resume lifecycle | Partially implemented | Local `run start/list/get/status/events/report/cancel/resume` exists. Gateway MCP now exposes `codencer.start_project_run`, `codencer.submit_project_task`, `codencer.list_project_runs`, `codencer.get_project_run_status`, `codencer.get_run_report`, `codencer.get_gateway_run_events`, and structured `cancel_project_run`/`resume_project_run` capability blockers. |
| Long-running tasks not dependent on one blocking request | Partially implemented | Local submit can run without `--wait`, Relay MCP has async project tools, and Gateway MCP now has a non-blocking async lifecycle. Gateway Console still submits through a blocking Relay call for immediate result rendering. |
| `get_run_report` for simple and manifest runs | Implemented for covered Gateway paths | Gateway tests cover submit/get report and manifest report paths. |
| Run state transitions include waiting/canceled/resumed | Partially implemented | Domain has states/gates in daemon tests; Gateway MCP preserves non-terminal `submitted/running` states and exposes structured cancel/resume blockers where project-level Relay support is absent. |

### 03 - Human Interrupts and Permissions

| Requirement | Status | Evidence |
| --- | --- | --- |
| Planning approval required | Partially implemented | Local blockers map manual approvals to `planning_approval_required` interrupt records; no complete UI/MCP approval lifecycle. |
| Clarifying questions | Partially implemented | Question blockers now produce `clarifying_question_required` interrupt records and Gateway `human_interrupt_created` audit; no first-class answer/resume command. |
| Permission requests | Partially implemented | Dangerous executor confirmation exists in Gateway Console, unsafe-action blockers map to `permission_request_required`, and Antigravity unsupported/out-of-workspace permission waits now become manual-attention results instead of timeouts; no generalized permission-request lifecycle. |
| OS/system human action required | Partially implemented | Daemon-not-running blockers map to `os_system_human_action_required` records; no full OS-action resolver flow. |
| Resume/cancel/audit interrupt lifecycle | Partially implemented | Local events include `human_interrupt_created`; Gateway audit records blocker interrupts; resume still returns a structured unsupported/capability blocker. |

### 04 - CLI Commands and Control Plane

| Requirement | Status | Evidence |
| --- | --- | --- |
| `codencer submit` local-first command | Partially implemented | Exists in `cmd/codencer/main.go`; default human output now shows sanitized result text and avoids local path/daemon URL leaks, but progress UX remains narrow. |
| `codencer run status` | Implemented | `run status` exists. |
| `codencer run events` | Implemented | `run events` returns local run timeline/events for known run plan records. |
| `codencer run report` | Implemented | `run report` returns the local run report without relying on a Gateway call. |
| `codencer run cancel` | Partially implemented | `run cancel` is exposed and returns structured local capability behavior where true cancellation is unsupported. |
| `codencer run resume` | Partially implemented | `run resume` is exposed as a structured unsupported blocker until daemon HTTP resume exists. |
| `codencer executor list/scan/test/default` | Implemented | Implemented in `cmd/codencer/main.go`. |
| `codencer sync` or publish equivalent | Partially implemented | `codencer sync status/preview/publish` exists with metadata-only preview and no raw upload. |
| Public defaults are local/self-host | Partially implemented | Config/default docs and scripts exist; needs re-check against new specs. |

### 05 - Executor Adapters and Client Proofs

| Requirement | Status | Evidence |
| --- | --- | --- |
| Codex real executor proof | Implemented | Current scoped proof `reports/public-selfhost-rc/20260624T120012Z` passed `real_executor_e2e_codex` with the configured Codex binary and `CODENCER_E2E_REQUIRED_REAL_EXECUTORS=codex`. |
| Claude Code real executor proof | Implemented | `reports/public-selfhost-rc/20260624T105654Z` passed `real_executor_e2e_claude` with the configured Claude Code binary. |
| Antigravity real executor proof | Partially implemented | The verifier now accepts isolated Antigravity instance metadata, requires the actual LS workspace to match the isolated verifier repo, and fails fast when it does not. Available local Antigravity LS candidates did not expose the isolated repo workspace; no passing proof exists. |
| Simulation guard | Implemented for current verifier | The live verifier checks generic simulated text, `is_simulation=true`, expected adapter/profile, real output/artifacts, and daemon simulation logs. |
| Fake never satisfies GO | Implemented for current verifier | Fake-only plumbing cannot produce `GO` when required real executor proofs are missing. |

### 06 - Run History, Audit, and Console

| Requirement | Status | Evidence |
| --- | --- | --- |
| Compact run history | Implemented | `/api/gateway/v1/runs` and `/console/runs` exist. |
| Run detail | Implemented | `/api/gateway/v1/runs/{id}` and `/console/runs/[id]` exist. |
| Audit lifecycle events | Implemented for Gateway-observed runs | Gateway records task/route/relay/connector/executor/start/terminal/report events and `human_interrupt_created` for blocker outcomes. |
| Pagination | Implemented for Gateway-observed history | Runs and audit support `limit`/`offset` and return `pagination.has_more`/`next_offset`; Console exposes previous/next controls. |
| Filters | Implemented for Gateway-observed history | Runs support project/status/scope; audit supports event type, project, run ID, and run history filters. |
| Grouped audit | Implemented for Gateway-observed history | Audit responses include grouped lifecycle summaries, and Console renders a grouped lifecycle section linking to run detail. |
| Local/synced/Gateway-submitted scopes | Partially implemented | Gateway run records expose `scope=gateway_submitted`; sync preview reports local run metadata as `scope=local`; confirmed sync publish records `scope=synced`. |
| Execution mode visible | Implemented for current UI | Gateway Console shows `Real executor`, `Simulation`, or `Unknown`. |

### 07 - Public vs Cloud Boundary

| Requirement | Status | Evidence |
| --- | --- | --- |
| Public/self-host defaults | Partially implemented | Public release checks exist. Needs re-audit after spec addition. |
| Private managed service code absent | Partially implemented | Public repo still has `internal/cloud` and `cmd/codencer-cloud*` packages; existing boundary docs classify them, but new spec may require stricter handling. |
| Hosted proof not claimed unless run | Partially implemented | Docs were previously cleaned; must re-run stale/boundary checks after new specs. |

## Immediate Blocking Items for GO

The release remains `NO-GO` until at least these are resolved:

1. Antigravity real executor proof must pass or the final verdict must remain `NO-GO`.
2. Async lifecycle has local, Relay MCP, and Gateway MCP coverage with structured blockers where unsupported; Gateway Console still needs a fully non-blocking submit/status polling mode before this area is fully complete.
3. Human interrupt lifecycle still needs complete operator answer/resume UI/MCP flows; first-class local interrupt records and Gateway audit now exist for blocker outcomes.
4. Full redaction proof across every CLI/MCP/UI/Gateway surface remains incomplete, although default local human CLI output and sync preview are now covered.
5. Raw log/artifact sync remains unsupported by design; only sanitized metadata-only `codencer sync publish --confirm` is implemented.
6. Broader incremental sync policy and external source reconciliation remain incomplete even though Gateway-observed and explicit synced metadata history now exist.
7. The final hardening report must end with exactly `Verdict: GO` or `Verdict: NO-GO`.
