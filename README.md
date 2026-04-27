<a id="codencer-the-tactical-orchestration-bridge"></a>

# Codencer: The Planner-to-Executor Bridge

Codencer is a planner-to-executor bridge (Tactical Orchestration Bridge) for coding work that needs an honest system of record.
Its core visible asset is the persisted execution trail: runs, steps, attempts, artifacts, validations, and gates.

Instead of planning for the operator, Codencer keeps execution local, isolates each attempt, records the state machine truth, and exposes the evidence a planner or human needs to decide what happens next.

> [!IMPORTANT]
> **Project Status: Open-source beta for the v2 local/self-host path (`v0.2.0-beta`)**.
> Codencer is publicly testable for the supported local, relay/runtime, cloud, planner/client, and provider tracks documented in [docs/BETA_TESTING.md](docs/BETA_TESTING.md). Compatibility-only and deferred surfaces remain explicitly outside that beta promise.

## Supported Beta Test Tracks

Use [docs/BETA_TESTING.md](docs/BETA_TESTING.md) as the repo-level tester guide. The quick chooser is:

| Track | Start doc | Build | Proof command | Current boundary |
| --- | --- | --- | --- | --- |
| Local-only daemon + CLI | [docs/SETUP.md](docs/SETUP.md) | `make build` | `./scripts/smoke_test_v1.sh` then `make smoke` | Canonical local proof is simulation-first; live adapter claims stay narrow. |
| Self-host relay / runtime | [docs/SELF_HOST_REFERENCE.md](docs/SELF_HOST_REFERENCE.md) | `make build` | `PLANNER_TOKEN=<planner-token> make self-host-smoke-mcp` | Canonical remote self-host path. |
| Self-host cloud control plane | [docs/CLOUD_SELF_HOST.md](docs/CLOUD_SELF_HOST.md) | `make build-cloud` | `make cloud-smoke` | Docker baseline and binary-native composed proof are separate. |
| Planner / client integrations | [docs/mcp/integrations.md](docs/mcp/integrations.md) | `make build build-cloud build-mcp-sdk-smoke` | `make flagship-planner-smoke` or self-host/cloud smoke with MCP/SDK enabled | ChatGPT-style and Claude Code-style operator lanes are packaged around the canonical remote MCP surfaces; universal product UI/auth support is not claimed. |
| Provider connectors | [docs/CLOUD_CONNECTORS.md](docs/CLOUD_CONNECTORS.md) | `make build-cloud` | `make cloud-smoke` plus provider tests | Slack is strongest; Jira is polling-first; the rest remain narrower. |

For a supported non-Docker repo pass:

```bash
make build-supported
make verify-beta
```

For the Docker-backed cloud baseline on a Docker-capable host:

```bash
make verify-beta-docker
```

---

## 🏛 The Bridge Doctrine

Codencer keeps the planner-to-executor bridge boundary explicit:

- the planner decides what to do
- the executor does the work
- Codencer owns the persistent execution truth between them

The state-of-record model is the product surface:

| Record | What it answers |
| --- | --- |
| Run | What larger unit of work is being tracked? |
| Step | What concrete task was submitted? |
| Attempt | What isolated execution instance tried to satisfy that step? |
| Artifact | What file, diff, prompt, or result payload was produced? |
| Validation | What test, lint, or policy check ran, and what happened? |
| Gate | Where does an operator need to approve, reject, or intervene? |

- **What it is**: A local control plane, state machine, validator, and evidence store for coding execution.
- **What it is not**: A planner, a chat UI, a workflow brain, or a generic remote shell surface.

```text
[ Planner ] ---- submit task ----> [ Codencer ] ---- execute ----> [ Adapter / Executor ]
     ^                                  |
     |                                  v
     +----- results, artifacts, validations, gates ---- persisted state of record
```

### Core Roles
- **Planner**: Human, script, chat UI, or remote client that decides what to do next.
- **Codencer**: Accepts the task, creates or locates the run and step, provisions the attempt workspace, enforces the runtime contract, and persists evidence.
- **Adapter / Executor**: Local execution path such as `codex`, `claude`, `qwen`, `antigravity*`, or `openclaw-acpx`.

## V2 Remote Path

The v2 path keeps the same control-plane split while adding a self-hostable remote surface:

```text
Planner / Chat
   -> Relay MCP / Planner API
   -> Relay Server
   -> Authenticated Connector (outbound websocket)
   -> Local Codencer Daemon
   -> Local Adapter / Executor
```

Key constraints remain unchanged:
- planning stays outside Codencer
- execution stays local
- the relay is transport and audit, not a planner
- the connector exposes only a narrow allowlisted proxy to the local daemon
- no raw remote shell or arbitrary filesystem surface is exposed

### New Binaries

- `bin/codencer-connectord`: enroll with a relay and maintain the outbound authenticated connector session
- `bin/codencer-relayd`: run the self-hostable relay server, planner-facing API, connector websocket endpoint, and relay-side MCP surface
- `bin/codencer-cloudctl`: admin CLI for cloud bootstrap, status, org/workspace/project, token, installation, runtime-connector, runtime-instance, event, and audit flows
- `bin/codencer-cloudd`: cloud control-plane server; can optionally start an internal relay runtime bridge for tenant-scoped Codencer runtime control
- `bin/codencer-cloudworkerd`: cloud worker for background connector maintenance; Jira is polling-first in the current beta track
- `bin/agent-broker`: build separately with `make build-broker` when you need the Windows-side agent-broker; it lives under the nested `cmd/broker` module

### Self-Host Quickstart

1. Build the main binaries with `make build`.
2. Build the Windows-side `agent-broker` separately with `make build-broker` if you need the Windows bridge.
3. Create a relay config and local planner token:
   `./bin/codencer-relayd planner-token create --config .codencer/relay/config.json --write-config --name operator --scope '*'`
4. Start the relay:
   `./bin/codencer-relayd --config .codencer/relay/config.json`
5. Start the local daemon near the repo with `make start` or `make start-sim`.
6. Mint a one-time enrollment token from the running relay:
   `./bin/codencer-relayd enrollment-token create --config .codencer/relay/config.json --label local-dev --json`
7. Enroll and run the connector in WSL/Linux next to the daemon:
   `./bin/codencer-connectord enroll --relay-url http://127.0.0.1:8090 --daemon-url http://127.0.0.1:8085 --enrollment-token <token>`
   `./bin/codencer-connectord run`
8. Inspect and control sharing explicitly:
   `./bin/codencer-connectord discover --config .codencer/connector/config.json`
   `./bin/codencer-connectord list`
   `./bin/codencer-connectord share --daemon-url http://127.0.0.1:8085`
   `./bin/codencer-connectord unshare --instance-id <instance-id>`
   `./bin/codencer-connectord config`
9. Inspect relay status, connectors, and advertised instances:
   `./bin/codencer-relayd status --config .codencer/relay/config.json`
   `./bin/codencer-relayd connectors --config .codencer/relay/config.json`
   `./bin/codencer-relayd instances --config .codencer/relay/config.json`
   `./bin/codencer-relayd audit --config .codencer/relay/config.json --limit 20`
10. Run the documented smoke path with `make self-host-smoke`, `make self-host-smoke-mcp`, or `make self-host-smoke-all` once the daemon and relay are already running. `self-host-smoke-mcp` includes the official MCP SDK proof helper; `self-host-smoke-all` adds share-control and multi-instance coverage.

Planner-facing relay routes live under `/api/v2`, and the relay-hosted MCP entrypoint is `/mcp` with `/mcp/call` kept as a compatibility path. Product-facing relay MCP deployments can also expose `/.well-known/oauth-protected-resource/mcp` and bearer `WWW-Authenticate` challenges by configuring `public_base_url` and `oauth_*` relay fields.
The connector now persists a local Ed25519 identity, `connector_id`, `machine_id`, and an explicit shared-instance allowlist under `.codencer/connector/config.json`.
The connector also persists a local `.codencer/connector/status.json` snapshot so operators can inspect session state, last heartbeat, and the currently shared instance set without contacting the relay.
Direct relay lookups for steps, artifacts, and gates now probe only authorized online instances and persist the discovered route, so planner HTTP and MCP flows do not depend on prior observation of those IDs.
Planner evidence retrieval through the relay now covers result, validations, logs, artifact lists, and artifact content.
For the end-to-end self-host flow and operating notes, see [docs/SELF_HOST_REFERENCE.md](docs/SELF_HOST_REFERENCE.md), [docs/CONNECTOR.md](docs/CONNECTOR.md), [docs/RELAY.md](docs/RELAY.md), and [docs/mcp/relay_tools.md](docs/mcp/relay_tools.md).

Daemon discovery and evidence notes:
- `GET /api/v1/instance` now exposes stable repo-local daemon identity plus manifest-backed discovery metadata.
- The daemon writes a repo-local instance manifest under `.codencer/instance.json` on startup and after Antigravity bind changes.
- `PATCH /api/v1/runs/{id}` remains best-effort abort. It returns success only when the active step actually reaches `cancelled`; otherwise Codencer leaves an explicit non-cancelled outcome and returns an error instead of claiming a hard kill.

### Cloud Control Plane (Beta Track)

The cloud surface is subordinate to the core bridge model. It adds tenancy, control-plane state, and provider-installation operations; it does not replace the local daemon, the relay bridge, or the run/step/attempt evidence path.

- Build the cloud binaries with `make build-cloud`.
- Start the cloud server with `./bin/codencer-cloudd --config .codencer/cloud/config.json`.
- Start it with `--relay-config` when you want cloud to claim and control Codencer runtime connectors and shared instances through the internal relay bridge.
- Use `./bin/codencer-cloudctl bootstrap` to seed org, workspace, project, membership, and API token state directly in the cloud store.
- Use `./bin/codencer-cloudctl status|orgs|workspaces|projects|memberships|tokens|install|runtime-connectors|runtime-instances|events|audit` for remote control-plane operations.
- Run `./bin/codencer-cloudworkerd` only when you have connector installations that need background polling. Jira is polling-first and requires `config.jql` or `config.project_key`; webhook ingest remains deferred in the current beta track.
- When cloud is running with the relay bridge, the cloud-scoped remote surface is:
  - HTTP under `/api/cloud/v1/runtime/*`
  - MCP under `/api/cloud/v1/mcp` with `/api/cloud/v1/mcp/call` kept as a compatibility alias
  - OAuth protected-resource metadata under `/.well-known/oauth-protected-resource/api/cloud/v1/mcp`
- Relay `/mcp` remains the self-host relay MCP surface. Cloud `/api/cloud/v1/mcp` is the tenant-scoped cloud contract.
- A Docker-based self-host baseline now lives under `deploy/cloud/` and can be smoke-checked with `make cloud-stack-smoke`.

For the cloud docs and status matrix, see [docs/CLOUD.md](docs/CLOUD.md), [docs/CLOUD_SELF_HOST.md](docs/CLOUD_SELF_HOST.md), and [docs/CLOUD_CONNECTORS.md](docs/CLOUD_CONNECTORS.md).

---

## 🚀 The Canonical "Day 0" Path (Human-First)

The shortest honest path is:

1. Clone a real git checkout and build the binaries.
2. Start the daemon in simulation or real mode.
3. Start a run.
4. Submit a step.
5. Wait for the step result.
6. Inspect artifacts, validations, and gates before deciding what happens next.

Concrete sequence:

1.  **Clone & Build**: `git clone` the repo → `make setup build`.
2.  **Start the Bridge**: `make start-sim` (for testing) or `make start` (for real agents).
3.  **Verify Instance**: `./bin/orchestratorctl instance --json` (Confirm repo and port).
4.  **Start a Mission**: `./bin/orchestratorctl run start <RUN_ID> <PROJECT>`.
5.  **Submit & Wait**: `./bin/orchestratorctl submit <RUN_ID> --goal "<text>" --wait --json`.
6.  **Audit the Truth**: `./bin/orchestratorctl step result <HANDLE>`.

### 📖 Operator Reference
> This file is for **AI Assistants (like Antigravity)** currently working **ON** the Codencer codebase.
> If you are a specialized **Planner** or **Machine-Operator** using the Codencer Bridge, you **MUST** refer to the **[AI Operator Guide](docs/AI_OPERATOR_GUIDE.md)** as your canonical source of truth for runtime operations.

- **[AI Operator Guide](docs/AI_OPERATOR_GUIDE.md)**: **Canonical Rules of Engagement** for AI assistants and automated planners (Runtime & Submission).
- **[CLI Automation Patterns](docs/CLI_AUTOMATION.md)**: Technical guide for JSON mode, exit codes, and sequential loops.
- **[Operator Runbook](docs/OPERATOR_RUNBOOK.md)**: Detailed step-by-step guidance for human operators.

---

### Core Guarantees

- **Step Isolation**: Each step executes in its own git worktree, preventing cross-task interference.
- **Immutable Evidence**: All logs, results, and artifacts are namespaced by Run, Step, and Attempt ID under `.codencer/artifacts/<run-id>/<step-id>/<attempt-id>/`, ensuring full auditability of repeated attempts.
- **Workspace Provisioning**: Codencer prepares attempt worktrees by copying configured files, wiring allowed symlinks, and running optional hooks.
- **Deterministic Submission Normalization**: Direct input is normalized into a canonical `TaskSpec`, and both the original input and normalized task are preserved as attempt artifacts.
- **Operator Control**: Gates, retries, approvals, and best-effort abort outcomes are explicit state transitions rather than hidden side effects.

> **Execution Path Note**: Codencer depends on Git Worktrees for isolating task attempts. Therefore, cloning the repository via `git clone` is the **only supported execution path**. Downloading a ZIP source archive will fail during targeted execution.

---

## ⚡️ Quickstart: Local Setup

Use this path when you want the canonical local beta flow and its evidence surfaces.

### 1. Build & Setup
```bash
# Initialize and build binaries
make setup build

# (Optional) Verify your local environment
./bin/orchestratorctl doctor
```

### 2. Start the Daemon
Choose your execution tier in `.env` (Simulation is enabled by default in `.env.example`):
```bash
# Start in Simulation Mode (Background)
make start-sim

# OR Start in Real Mode (Requires agent binaries like codex or claude)
# Edit .env: ALL_ADAPTERS_SIMULATION_MODE=0
make start
```

For Claude, Codencer invokes the installed CLI as `claude -p --output-format json`, sends the step prompt on `stdin`, and runs from the isolated attempt workspace as the process `cwd`.

The Claude adapter wrapper path is implemented and test-covered in this repo: prompt shaping, normalization, lifecycle behavior, fake-binary integration, and simulation conformance are exercised, but the repo test suite does not run a live authenticated Claude service call. Treat `/api/v1/compatibility` plus your actual runtime environment as the source of truth for local adapter readiness.

For Codex, Codencer invokes the installed CLI as `codex exec` by default, sends the task prompt on `stdin`, and writes a normalized `result.json` from Codex's final message when the CLI does not write one itself. Set `CODEX_BINARY` only if `codex` is outside your `$PATH`; set `CODEX_ADAPTER_MODE=legacy-agent` only for the older `codex-agent` wrapper.

`/api/v1/compatibility` is a runtime diagnostic surface, not a beta-support certificate. It reports current binary availability, simulation mode, and local binding state; it does not promote an adapter into the beta promise by itself.

### 3. Run Your First Tactical Task
Submit a task, wait for the step, then inspect the recorded result. For the full local operator sequence, see the **[Canonical Local Runbook](docs/EXAMPLES.md)**.

```bash
# 1. Start a new mission (System of Record)
./bin/orchestratorctl run start first-run my-project

# 2a. Submit a rich TaskSpec file and wait for completion
./bin/orchestratorctl submit first-run examples/tasks/bug_fix.yaml --wait --json

# 2b. Or use direct convenience input for local automation
./bin/orchestratorctl submit first-run --goal "Fix the failing tests in pkg/foo" --title "Fix pkg/foo tests" --adapter codex --wait --json

# 3. View the Authoritative Truth (The Summary)
# Note: Use the Step UUID Handle printed after submission
./bin/orchestratorctl step result <UUID>
```

For the repo-proven legacy same-run local parity path, run the six-input smoke directly. If no daemon is already reachable, the script auto-starts a temporary simulation daemon and exercises the current local wait/result contract end to end:

```bash
./scripts/smoke_test_v1.sh
./scripts/smoke_test_v1.sh
make smoke
```

### 3.2 Standard Submission Flows

Codencer supports both structured and convenience input via terminal:

#### A. Multiline Text Prompt (Heredoc)
Ideal for large, human-readable prompts without creating a file:
```bash
cat <<'EOF' | ./bin/orchestratorctl submit run-01 --stdin --title "Fix Lints" --adapter codex --wait --json
Fix all lint errors in the internal/app package.
Exclude the test files.
Use the 'go-lint' tool.
EOF
```

#### B. JSON Task String (Pipe)
Ideal for machine-to-machine hand-offs:
```bash
echo '{"version":"v1","goal":"Update README"}' | ./bin/orchestratorctl submit run-01 --task-json - --wait --json
```

#### C. Broker-Backed Execution
Directly target an IDE-bound agent via the agent-broker bridge using direct input:
```bash
./bin/orchestratorctl submit run-01 --goal "Check UI" --adapter antigravity-broker --wait --json
```

#### OpenClaw ACPX
Relay tasks to an OpenClaw-compatible executor via the standardized ACP bridge. Use `--wait --json` for synchronous machine-safe handoffs.
```bash
./bin/orchestratorctl submit my-run \
  --goal "Fix UI layout issues in the landing page" \
  --adapter openclaw-acpx \
  --wait --json
```

---

## 🔍 The Audit Trail (Authoritative Evidence)

The fastest way to understand Codencer is to inspect the evidence surfaces in order:

1.  **Authoritative Summary**: `step result <UUID>` shows the persisted run/step outcome.
2.  **Raw Execution Trail**: `step logs <UUID>` shows the executor output captured for that step attempt.
3.  **Audit Evidence**: `step artifacts <UUID>` and `step validations <UUID>` show the files and checks Codencer recorded.

- **`completed`**: Goal met, all tests passed.
- **`completed_with_warnings`**: Success, but with non-critical issues (lint/tests).
- **`failed_terminal`**: Goal not met (e.g. tests failed). Review validations.
- **`timeout`**: Execution exceeded limits. Review logs for hangs.
- **`cancelled`**: Manually stopped by the operator.
- **`needs_approval`**: Policy gate hit; awaiting operator intervention.
- **`needs_manual_attention`**: System ambiguity or crash. Review daemon/agent logs.

### Auditing the Evidence
Every task execution leaves a permanent audit trail:
1. **Summary**: Run `./bin/orchestratorctl step result <id>` for the high-level spec.
2. **Logs**: Run `./bin/orchestratorctl step logs <id>` for the raw agent stdout.
3. **Artifacts**: Every modified file and diff is stored in `.codencer/artifacts/`. Use `./bin/orchestratorctl step artifacts <id>` to see the exact paths and SHA-256 hashes.
4. **Validations**: Run `./bin/orchestratorctl step validations <id>` to see specific test/lint results.

For Claude attempts specifically, the standard evidence set is:
- `prompt.txt`: the exact prompt Codencer built and sent to Claude
- `stdout.log`: raw Claude JSON output
- `stderr.log`: raw Claude stderr
- `result.json`: synthesized normalized Codencer result

## 🧾 Submission Inputs

Codencer supports two submit styles:

1. **Canonical TaskSpec**: submit a full YAML or JSON task definition when you need rich structure.
2. **Direct convenience input**: submit a prompt or goal directly and let the CLI deterministically normalize it into `TaskSpec`.

Direct input is intentionally narrow. It does not plan, decompose work, merge multiple sources, or invent strategy.

### Exactly One Primary Source

`submit` requires exactly one of:
- positional task file
- `--task-json <path|->`
- `--prompt-file <path>`
- `--goal <text>`
- `--stdin`

Direct metadata flags are only supported with `--prompt-file`, `--goal`, and `--stdin`:
- `--title`
- `--context`
- `--adapter`
- `--timeout`
- `--policy`
- repeated `--acceptance`
- repeated `--validation`

### Deterministic Defaults

For direct convenience input:
- `version` defaults to `v1`
- `run_id` comes from the CLI `<RUN_ID>`
- `title` comes from `--title`, otherwise the prompt filename basename, otherwise `Direct task`
- `goal` is the exact submitted text from `--goal`, `--prompt-file`, or `--stdin`
- repeated `--validation` flags become deterministic validation commands named `validation-1`, `validation-2`, and so on

`context` and `acceptance` are preserved in the normalized task and provenance, but they are currently retained metadata rather than separate executor-driving runtime fields.

### Provenance and Auditability

Codencer maintains a complete audit trail for every task attempt. Every accepted submission persists:
- **`original-input.*`**: The exact content submitted from any source (file, STDIN, prompt).
- **`normalized-task.json`**: The deterministic `TaskSpec` Codencer actually executed.

These are recorded as immutable artifacts under the attempt root (`.codencer/artifacts/...`) and are visible through normal artifact inspection, allowing auditors to verify both the intent and the execution of any automated task.

## 🔁 Ordered Task Lists

The official v1 sequential-execution story is wrapper-based:
- start or reuse a run
- submit one item at a time with `submit --wait --json`
- inspect the exit code and terminal payload outside Codencer
- decide whether to continue or stop outside Codencer

Official wrapper examples live in [examples/automation](examples/automation):
- [run_tasks.sh](examples/automation/run_tasks.sh)
- [run_tasks.ps1](examples/automation/run_tasks.ps1)
- [run_tasks.py](examples/automation/run_tasks.py)

This keeps Codencer narrow as a bridge rather than a workflow brain.

For a deeper dive into agent installation and advanced configuration, see the **[Environmental Reference Guide](docs/SETUP.md)**.

---

## 🛡 Why Codencer?

Codencer exists to keep tactical coding work inspectable and recoverable:

1. **Workspace Safety**: Agents run in isolated Git Worktrees. Diffs are captured and validated before any commit.
2. **Audit-Proof Ledger**: Every attempt is recorded in a local SQLite database (embedded via CGO) with SHA-256 hashes of all artifacts.
3. **Idempotent Recovery Posture**: Interrupted tasks can be retried or analyzed from persisted run, step, and attempt state.
4. **Validation-First Outcomes**: Tasks only complete when the recorded validation contract passes.
5. **Remote Control Without Remote Execution**: Relay and cloud surfaces expose narrow planner APIs while execution and artifacts remain on the daemon side.

---

## ⚠️ Known Limitations (Public Beta)

Codencer is beta-track ready within the documented surfaces, but the repo keeps several boundaries explicit:
- **No Planner In Core**: Codencer never decomposes, prioritizes, or decides strategy. The planner still owns those decisions.
- **Best-Effort Abort**: `PATCH /api/v1/runs/{id}` and relay abort flows are honest but not universal hard-kill guarantees. A run is only reported cancelled when the adapter actually stops.
- **Opportunistic Remote Routing**: Relay step, gate, and artifact routing still learns and persists route hints opportunistically, but direct remote lookups also probe authorized online shared instances before failing closed.
- **Bounded Artifact Transport**: Connector transport rejects oversized artifact bodies instead of turning the relay into a bulk file tunnel. Large binary transfer is intentionally limited.
- **Static Self-Host Auth**: Planner auth is static bearer-token based, suitable for narrow self-host beta use but not enterprise IAM.
- **Ordered Execution Is Wrapper-Based**: Sequential workflows remain wrapper- or planner-driven outside Codencer core.

For the consolidated operator-facing boundary list, see [docs/KNOWN_LIMITATIONS.md](docs/KNOWN_LIMITATIONS.md).

### Runtime Capability Truth

Adapter availability is runtime-derived, not a hardcoded support matrix. The source of truth is:
- `GET /api/v1/compatibility`
- `GET /api/v1/instance`
- `./bin/orchestratorctl instance --json`

Those surfaces reflect actual registered adapters, simulation mode, binary availability, and Antigravity binding state at runtime.

### WSL / Windows / Antigravity

The practical cross-side model is:
- daemon, repos, worktrees, and artifacts in WSL/Linux
- connector on the same side as the daemon by default
- agent-broker and IDE on Windows when needed
- relay as a separate remote control plane only

Use `orchestratorctl antigravity bind <PID>` to bind this repo to an active Antigravity instance. Binding selects the repo-scoped target, but execution still stays local and still depends on the chosen adapter profile.

For the full trust-boundary and topology guidance, see [docs/WSL_WINDOWS_ANTIGRAVITY.md](docs/WSL_WINDOWS_ANTIGRAVITY.md).

### 🔍 Terminal Step States
Codencer distinguishes between different failure modes to help you recover faster:

| State | Meaning | Typical Recovery |
| :--- | :--- | :--- |
| `completed` | Success: All goals and validations met. | Next step. |
| `failed_validation` | Validations failed: Agent finished but tests/lint failed. | Fix code/prompt. |
| `failed_adapter` | Agent crashed: The binary or process failed. | Check config/keys. |
| `failed_bridge` | Bridge error: Orchestrator infrastructure failure. | Check disk/git/locks. |
| `timeout` | Time limit exceeded: Process was killed. | Increase timeout. |
| `cancelled` | Explicit stop: Operator aborted the run. | Resubmit if needed. |

## 🧪 Simulation vs. Real Execution

1. **Simulation Mode** (`make start-sim`): Validates the daemon, state machine, CLI, and evidence path without requiring a live executor.
2. **Real Mode**: Exercises the end-to-end loop with real agents. `codex` is the primary intended local beta adapter; the checked-in proof now covers the default `codex exec` invocation with a fake binary, while live authenticated Codex execution remains operator-environment proof.

Current local adapter proof is intentionally narrow:
- `codex`: primary intended local beta adapter; repo proof covers simulation plus fake-binary `codex exec` invocation, stdin prompt delivery, and result synthesis. Live authenticated Codex service calls are not exercised in repo tests.
- `claude`: strongest adapter-specific wrapper proof in repo, but still fake-binary and non-authenticated.
- `qwen`: simulation/conformance proof only; kept as secondary.
- `antigravity` and `antigravity-broker`: secondary, environment-specific proof only.
- `openclaw-acpx` and `ide-chat`: experimental/deferred, not part of the local beta promise.
- daemon-local `/mcp/call`: compatibility/admin bridge only, not the public planner MCP contract.

---

## 📖 Documentation

Use the following docs to choose the right supported surface quickly.

### ⚡️ User Guidance (Start Here)
- **[Public Beta Test Tracks](docs/BETA_TESTING.md)** — Fastest way to choose the right supported test lane and command set.
- **[Environmental Reference](docs/SETUP.md)** — Setup hub, platform chooser, native Linux guidance, and track routing.
- **[Known Limitations](docs/KNOWN_LIMITATIONS.md)** — Consolidated operator-grade beta boundaries from current repo truth.
- **[Operator Runbook](docs/OPERATOR_RUNBOOK.md)** — The canonical "Day 0" flow for humans.
- **[AI Operator Guide](docs/AI_OPERATOR_GUIDE.md)** — Canonical rules for AI planners and assistants.
- **[CLI Automation Patterns](docs/CLI_AUTOMATION.md)** — Machine-safe JSON mode and sequential loops.
- **[Self-Host Relay / Runtime Guide](docs/SELF_HOST_REFERENCE.md)** — End-to-end relay/connector operator flow.
- **[Self-Host Cloud Control Plane Guide](docs/CLOUD_SELF_HOST.md)** — Bootstrap, smoke, and composed cloud runtime guidance.
- **[Planner / Client Integration Notes](docs/mcp/integrations.md)** — Relay/cloud HTTP + MCP compatibility matrix.
- **[Cloud Connector Matrix](docs/CLOUD_CONNECTORS.md)** — Per-provider install/test depth and limitations.
- **[Troubleshooting Guide](docs/TROUBLESHOOTING.md)** — Resolving infrastructure vs goal failures.
- **[Architecture Overview](docs/02_architecture.md)** — Current daemon, connector, relay, and trust-boundary model.
- **[WSL / Windows / Antigravity Topology](docs/WSL_WINDOWS_ANTIGRAVITY.md)** — Practical cross-side deployment guidance.

### 🛠 Project Governance & Maintenance (Internal)
- **[Gap Audit & Roadmap](docs/internal/GAP_AUDIT.md)** — Current V1 release blockers and debt.
- **[Development Progress](docs/internal/PROGRESS.md)** — Historical and current technical timeline.
- **[Technical Task Backlog](docs/internal/TASKS.md)** — Detailed micro-task status for maintainers.
- **[Contributing Guide](CONTRIBUTING.md)** — How to set up a dev environment and submit PRs.

---

## ⚖ License

Codencer is released under the **MIT License**. See the [LICENSE](LICENSE) file for the full text.

## 🏗 One-Repo-One-Instance Model

Codencer is designed around an explicit, repo-bound execution model:
- **1 Git Clone = 1 Daemon Instance**: Each repository checkout manages its own ledger and workspaces.
- **Explicit Targeting**: Start the daemon with `--repo-root <path>` to anchor all relative state (DB, artifacts) to that project, regardless of the startup directory.
- **Multi-Instance Support**: To run multiple instances on the same machine, use different ports and repo roots:
  ```bash
  ./scripts/start_instance.sh ~/projects/project-a 8085
  ./scripts/start_instance.sh ~/projects/project-b 8086
  ```
- **Identity Verification**: Always use `./bin/orchestratorctl instance --json` to verify which repository and port a daemon is serving before submitting tasks.

For more details, see **[Setup & Multi-Instance Workflows](docs/SETUP.md)**.
