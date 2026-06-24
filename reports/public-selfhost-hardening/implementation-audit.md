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
| Local-first source of truth | Partially implemented | Local daemon/CLI exists; default init/config/project/status/run/submit human output is redacted, while explicit JSON/debug/path outputs still carry local state for operator tooling. |
| Explicit sync/publish | Partially implemented | `codencer sync status/preview/publish` now provides metadata-only preview; confirmed publish ingests sanitized metadata into Gateway run history and records aggregate/per-run sync audit events. Raw logs/artifacts remain blocked. |
| Local CLI submit UX | Partially implemented | `codencer submit` exists and is local-first; default human output redacts local paths and now shows local lifecycle progress for run id, submitted step/profile, task status, terminal result, report-store availability, and non-terminal `codencer run report <run_id>` follow-up. Broader interactive/progress streaming remains narrow. |
| Async run lifecycle | Partially implemented | Local `run start/list/get/status/events/report/cancel/resume` exists; local resume now routes through daemon `RecoveryService.ResumeRun` for `created` and `paused_for_gate` runs and returns structured blockers for non-resumable or missing-run states. Gateway/Relay/Connector now route true project-scoped cancel and project-scoped resume, Gateway MCP exposes async start/submit/list/status/report/events/cancel/resume, successful resumable project resumes produce `run_resumed`, and non-resumable project resumes produce structured requested/blocked audit events. Gateway Console now submits simple tasks and advanced manifest/run-plan tasks with `wait=false`, polls run reports, and records terminal audit events on report refresh. |
| Human interrupt lifecycle | Partially implemented | Local reports/events now expose first-class `human_interrupts`, local and project-level daemon-backed resume exist for resumable states, Gateway blocker outcomes emit `human_interrupt_created` audit events, Gateway HTTP/MCP and Console run detail can record sanitized operator responses as `human_interrupt_responded`, explicit `follow_up=resume/cancel` uses stored safe route metadata to attempt resume/cancel and audit the requested/resumed/cancelled/blocked outcome, non-resumable resume attempts record requested/blocked audit events, and Antigravity unsafe permission waits now fail fast as manual-attention results; broader planner/executor continuation after arbitrary answer/approval/permission responses remains incomplete. |
| Real executor proofs | Partially implemented | Codex has prior artifact-backed proof and latest rerun invoked the real Codex binary with simulation disabled but failed on an external Codex usage-limit error; earlier Claude Code proof exists; Antigravity remains unproven and now fails early when the provided LS workspace does not match the isolated verifier repo. |
| Run history/audit/console | Partially implemented | Gateway-observed run history/audit now includes scope, limit/offset pagination, server-side filters, grouped lifecycle summaries, and explicit synced metadata audit events; broader synced/local ingest transport remains incomplete. |
| Redaction | Partially implemented | Gateway/sync sanitization exists and artifact-backed release verification now covers default human CLI output for init, config show, config profile/set commands, project init/status/scan, executor list/scan/test/default, setup self-host/relay, activation self-host, sync preview, submit, run events, run report, and run resume blocker output. Source-tree and unpacked-artifact Gateway smoke now also sweeps public Gateway API outputs for relays, projects, machines, connectors, executors, runs, run detail/events, audit events, and activation commands. Broader explicit JSON/debug/path surface policy proof is still incomplete. |
| Public/private boundary | Implemented for current public boundary | Public boundary docs classify self-host/community cloud-control-plane primitives separately from private managed-service candidates. `scripts/check_public_boundary.py` now scans active deploy files for stale release labels, rejects commercial endpoints as public defaults, rejects tracked runtime state/secrets/local paths, checks release artifacts, and rejects source/deploy/script/web paths that match obvious private managed-service categories such as billing, metering, KMS/Vault, managed runners, support/admin consoles, marketplace submission material, or official connector credentials. |
| Public RC verifier | Partially implemented | `make verify-public-selfhost-rc` emits only `GO`/`NO-GO`, requires configured real-proof coverage, reports `NO-GO` when required proofs are missing, and public boundary checks reject stale active docs claiming `PARTIAL` verdicts; Antigravity remains unproven. |

## Requirement Audit

### 00 - Public Self-host Release Gate

| Requirement | Status | Evidence |
| --- | --- | --- |
| Final verdicts only `GO` or `NO-GO` | Implemented | `scripts/verify_public_selfhost_rc.sh` emits `GO` or `NO-GO`; no `PARTIAL` branch remains, active docs now describe missing real proof as `NO-GO`, and `scripts/check_public_boundary.py` rejects stale `reports PARTIAL` claims plus malformed final hardening-report verdict lines. |
| Fake/simulation cannot satisfy GO | Implemented for current verifier | Real executor gates reject simulation text/metadata and missing required real proofs force `NO-GO`; Codex and Claude real gates passed with simulation disabled. |
| Artifact-backed verifier | Implemented | `make verify-public-selfhost-rc` builds/unpacks artifacts through `scripts/verify_public_selfhost_rc.sh`. |
| Codex, Claude Code, and Antigravity real proofs | Partially implemented | Codex passed current artifact-backed scoped proof in `reports/public-selfhost-rc/20260624T120012Z`; Codex and Claude Code passed earlier artifact-backed real gates in `reports/public-selfhost-rc/20260624T105654Z`; Antigravity remains missing. |
| Missing reports/audit/path leaks fail gate | Partially implemented | Existing Gateway live verifier checks report/audit/leaks and now asserts run/audit pagination plus grouped audit arrays; source/artifact Gateway smoke sweeps core public Gateway API outputs for path/token/unsafe-field leaks; human interrupt audit is covered for blocker outcomes, but multi-executor proof coverage remains incomplete. |
| Machine-readable and human report | Implemented | `reports/public-selfhost-rc/<timestamp>/summary.json` and `.md` are produced by the RC script. |

### 01 - Local-first Source of Truth

| Requirement | Status | Evidence |
| --- | --- | --- |
| Manual local CLI runs stay local by default | Partially implemented | `codencer submit` calls `internal/localexec.Service.Submit`; no Gateway dependency by default. |
| Gateway is control plane/index/sync target, not global source of truth | Partially implemented | Gateway records Gateway-observed run history; local sync preview reports `scope=local`; confirmed sync publish creates sanitized `scope=synced` history records. |
| Raw logs/artifacts not uploaded by default | Partially implemented | Gateway sanitizes report JSON; `codencer sync` is metadata-only and blocks raw artifact/log upload. Local reports can still contain local refs on disk. |
| Explicit sync/publish behavior | Partially implemented | `codencer sync status/preview/publish` exists; publish requires `--confirm`, requires login, blocks raw artifact/log requests, sends only sanitized metadata, and Gateway records sanitized `sync.publish`/`sync.run_published` audit events. |
| Default output does not leak local paths | Partially implemented | Default human output for init, config show, config profile/set commands, project init/status/scan, executor list/scan/test/default, setup self-host/relay, activation self-host, sync preview, submit, run events, run report, and run resume blocker output is redacted and tested; source/artifact Gateway API outputs are now checked for local path, daemon URL, token, and unsafe field leaks. Explicit `--json` and path/debug commands still include local `repo_root`, `daemon_url`, and `report_path` for operator tooling. |

### 02 - Execution Lifecycle

| Requirement | Status | Evidence |
| --- | --- | --- |
| Submit/status/events/report/cancel/resume lifecycle | Partially implemented | Local `run start/list/get/status/events/report/cancel/resume` exists, and local resume succeeds for daemon-resumable `created` and `paused_for_gate` states while returning structured blockers for non-resumable or missing-run states. Gateway MCP now exposes `codencer.start_project_run`, `codencer.submit_project_task`, `codencer.list_project_runs`, `codencer.get_project_run_status`, `codencer.get_run_report`, `codencer.get_gateway_run_events`, project-scoped `codencer.cancel_project_run`, and project-scoped `codencer.resume_project_run`; successful resumable project resumes produce `run_resumed`, while non-resumable resumes record `resume_project_run_requested` plus `resume_project_run_blocked` audit events. |
| Long-running tasks not dependent on one blocking request | Partially implemented | Local submit can run without `--wait`, Relay MCP has async project tools, Gateway MCP has a non-blocking async lifecycle, and Gateway Console simple-task plus advanced manifest/run-plan submit now return after submission with `wait=false` and poll reports until terminal evidence is available. Blocking convenience wrappers remain available for deterministic smoke paths. |
| `get_run_report` for simple and manifest runs | Implemented for covered Gateway paths | Gateway tests cover submit/get report and manifest report paths. |
| Run state transitions include waiting/canceled/resumed | Partially implemented | Domain has states/gates in daemon tests; local daemon/CLI resume records `run_resumed` for daemon-resumable states and `run_resume_blocked` for non-resumable states. Gateway MCP preserves non-terminal `submitted/running` states, forwards project-scoped cancel/resume, records `run_cancelled` and `run_resumed` when downstream execution supports it, and preserves structured resume blockers for non-resumable states. |

### 03 - Human Interrupts and Permissions

| Requirement | Status | Evidence |
| --- | --- | --- |
| Planning approval required | Partially implemented | Local blockers map manual approvals to `planning_approval_required` interrupt records; no complete UI/MCP approval lifecycle. |
| Clarifying questions | Partially implemented | Question blockers now produce `clarifying_question_required` interrupt records and Gateway `human_interrupt_created` audit; Gateway HTTP/MCP and Console run detail can record a sanitized operator answer and an explicit `follow_up=resume/cancel` can attempt project resume/cancel through the stored route, while broader planner/executor continuation after arbitrary answers remains incomplete. |
| Permission requests | Partially implemented | Dangerous executor confirmation exists in Gateway Console, unsafe-action blockers map to `permission_request_required`, and Antigravity unsupported/out-of-workspace permission waits now become manual-attention results instead of timeouts; no generalized permission-request lifecycle. |
| OS/system human action required | Partially implemented | Daemon-not-running blockers map to `os_system_human_action_required` records; no full OS-action resolver flow. |
| Resume/cancel/audit interrupt lifecycle | Partially implemented | Local events include `human_interrupt_created`, `run_resumed`, and `run_resume_blocked`; Gateway audit records blocker interrupts, sanitized operator responses from HTTP/MCP/Console, explicit follow-up resume/cancel attempts, project resume requested/blocked events for non-resumable states, and `run_resumed` or `run_cancelled` when downstream project resume/cancel succeeds; project-scoped cancel and resume are forwarded and audited. |

### 04 - CLI Commands and Control Plane

| Requirement | Status | Evidence |
| --- | --- | --- |
| `codencer submit` local-first command | Partially implemented | Exists in `cmd/codencer/main.go`; default human output now shows sanitized lifecycle progress, result text, and local report-store guidance while avoiding local path/daemon URL leaks. Broader interactive/progress streaming remains narrow. |
| `codencer run status` | Implemented | `run status` exists. |
| `codencer run events` | Implemented | `run events` returns local run timeline/events for known run plan records. |
| `codencer run report` | Implemented | `run report` returns the local run report without relying on a Gateway call. |
| `codencer run cancel` | Partially implemented | `run cancel` is exposed locally, and project-scoped Gateway/Relay/Connector cancel now reaches daemon-backed cancellation; executor-specific cancellation behavior still depends on the active executor/daemon state. |
| `codencer run resume` | Partially implemented | `run resume` now calls daemon HTTP resume and succeeds for `created` or `paused_for_gate` runs supported by `RecoveryService.ResumeRun`; non-resumable and missing daemon-run records return a structured blocker with `run_resume_blocked`, and Gateway MCP resume attempts emit sanitized requested/blocked audit events for run-history correlation. |
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
| Audit lifecycle events | Implemented for Gateway-observed runs | Gateway records task/route/relay/connector/executor/start/terminal/report events, `human_interrupt_created` for blocker outcomes, sanitized `human_interrupt_responded` operator responses, project resume requested/blocked events for non-resumable states, and `run_resumed` when downstream project resume succeeds. |
| Pagination | Implemented for Gateway-observed history | Runs and audit support `limit`/`offset` and return `pagination.has_more`/`next_offset`; Console exposes previous/next controls. |
| Filters | Implemented for Gateway-observed history | Runs support project/status/scope; audit supports event type, project, run ID, and run history filters. |
| Grouped audit | Implemented for Gateway-observed history | Audit responses include grouped lifecycle summaries, and Console renders a grouped lifecycle section linking to run detail. |
| Local/synced/Gateway-submitted scopes | Partially implemented | Gateway run records expose `scope=gateway_submitted`; sync preview reports local run metadata as `scope=local`; confirmed sync publish records `scope=synced` run history and sanitized sync audit events. |
| Execution mode visible | Implemented for current UI | Gateway Console shows `Real executor`, `Simulation`, or `Unknown`. |

### 07 - Public vs Cloud Boundary

| Requirement | Status | Evidence |
| --- | --- | --- |
| Public/self-host defaults | Implemented for current boundary | `make verify-public-release` passes, public default files are checked for commercial endpoints, and active self-host cloud deploy defaults now use `v0.3.0-local-prod-rc.1` instead of stale beta labels. |
| Private managed service code absent | Implemented for current boundary | Self-host/community cloud-control-plane packages remain public and documented as non-managed-service primitives. The boundary checker rejects obvious private managed-service source/deploy path classes and production hosted endpoints/defaults. |
| Hosted proof not claimed unless run | Implemented for current boundary | `scripts/check_public_boundary.py` rejects stale active release labels and mixed public self-host RC verdict language; current final report remains `NO-GO` where live proof is missing. |

## Immediate Blocking Items for GO

The release remains `NO-GO` until at least these are resolved:

1. Antigravity real executor proof must pass or the final verdict must remain `NO-GO`.
2. Async lifecycle now covers local, Relay MCP, Gateway MCP, Gateway Console simple-task and advanced manifest/run-plan submit/report polling, project-scoped cancel, and local/project-level daemon-backed resume for resumable states; non-resumable project resume still returns structured blockers.
3. Human interrupt lifecycle still needs broader planner/executor continuation after arbitrary answer/approval/permission responses; first-class local interrupt records plus local and project-level resume/cancel, explicit Gateway `follow_up=resume/cancel`, Gateway HTTP/MCP/Console response audit, and non-resumable resume requested/blocked audit now exist for blocker outcomes.
4. Full redaction proof across every CLI/MCP/UI/Gateway surface remains incomplete, although default local human CLI output for init, config show, config profile/set commands, project init/status/scan, executor list/scan/test/default, setup self-host/relay, activation self-host, sync preview, submit, run events, run report, run resume blocker output, and core source/artifact Gateway API outputs are now covered.
5. Raw log/artifact sync remains unsupported by design; only sanitized metadata-only `codencer sync publish --confirm` is implemented.
6. Broader incremental sync policy and external source reconciliation remain incomplete even though Gateway-observed and explicit synced metadata history/audit now exist.
7. The final hardening report must end with exactly `Verdict: GO` or `Verdict: NO-GO`.
