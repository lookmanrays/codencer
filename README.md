# Codencer: The Tactical Orchestration Bridge

Codencer is a tactical orchestration bridge that manages execution, isolation, and high-fidelity audit trails for coding agents. It serves as the **system of record** between a high-level **Planner** (human or LLM) and tactical **Coding Agents** (Codex, Claude).

Designed for **local-first, self-hosted developer toolchains**, Codencer provides the missing "relay" layer that ensures every task attempt is isolated, provisioned, and validated before it ever reaches your production branch.

> [!IMPORTANT]
> **Project Status: Open-source alpha for the v2 local/self-host path (`v0.2.0-alpha`)**.
> Codencer is coherent and buildable for disciplined local and self-host use, but the current relay, auth, artifact transport, and cancellation guarantees are still alpha-grade and documented honestly below.

---

## 🏛 The Bridge Doctrine

Codencer is a **Tactical Orchestration Bridge**, not a strategic planner. It handles the **Execution Layer** (isolation, provisioning, monitoring, and evidence) while the **Brain Layer** (human or LLM) handles strategy and decision-making.

- **What it is**: A system of record, a workspace isolator, a validator, and a provider of immutable artifacts.
- **What it is not**: A planner, a chat UI, a generic cloud orchestration service, or an AI "agent" that thinks about what to do next.

```text
[ Planner (Brain) ] <---------- (ResultSpec) ---------+
       |                                              |
   (TaskSpec)                                   [ Bridge (Codencer) ]
       |                                              |
       +-------------------> [ Agent (Worker) ] <-----+
                              (File Edits)
```

### Core Roles
- **Planner (Brain)**: You, a Chat UI, or an agentic planner. Decides **what** to do.
- **Bridge (Codencer)**: Receives the `TaskSpec`, manages workspace isolation (Git Worktrees), enforces policies, and monitors execution.
- **Coding Agent (Worker)**: The tactical tool performing the actual work (e.g., `codex-agent`, `claude`).

## V2 Remote Path

Codencer now includes the first open-source self-hostable remote path while keeping execution local:

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
- `bin/codencer-cloudworkerd`: cloud worker for background connector maintenance; Jira is polling-first in this alpha pass
- `bin/agent-broker`: build separately with `make build-broker` when you need the Windows-side agent-broker; it lives under the nested `cmd/broker` module

### Cloud Control Plane (Alpha)

Codencer also includes a cloud control-plane foundation for provider connector installations, operator bootstrap, and tenant-scoped Codencer runtime control. It is not a replacement for the local daemon, the relay bridge, or the self-host run/step/attempt path.

- Build the cloud binaries with `make build-cloud`.
- Start the cloud server with `./bin/codencer-cloudd --config .codencer/cloud/config.json`.
- Start it with `--relay-config` when you want cloud to claim and control Codencer runtime connectors and shared instances through the internal relay bridge.
- Use `./bin/codencer-cloudctl bootstrap` to seed org, workspace, project, membership, and API token state directly in the cloud store.
- Use `./bin/codencer-cloudctl status|orgs|workspaces|projects|memberships|tokens|install|runtime-connectors|runtime-instances|events|audit` for remote control-plane operations.
- Run `./bin/codencer-cloudworkerd` only when you have connector installations that need background polling. Jira is polling-first and requires `config.jql` or `config.project_key`; webhook ingest is not implemented for Jira in this pass.
- When cloud is running with the relay bridge, the cloud-scoped remote surface is:
  - HTTP under `/api/cloud/v1/runtime/*`
  - MCP under `/api/cloud/v1/mcp` with `/api/cloud/v1/mcp/call` kept as a compatibility alias
- Relay `/mcp` remains the self-host relay MCP surface. Cloud `/api/cloud/v1/mcp` is the tenant-scoped cloud contract.
- A Docker-based self-host baseline now lives under `deploy/cloud/` and can be smoke-checked with `make cloud-stack-smoke`.

For the cloud docs and status matrix, see [docs/CLOUD.md](docs/CLOUD.md), [docs/CLOUD_SELF_HOST.md](docs/CLOUD_SELF_HOST.md), and [docs/CLOUD_CONNECTORS.md](docs/CLOUD_CONNECTORS.md).

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

Planner-facing relay routes live under `/api/v2`, and the relay-hosted MCP entrypoint is `/mcp` with `/mcp/call` kept as a compatibility path.
The connector now persists a local Ed25519 identity, `connector_id`, `machine_id`, and an explicit shared-instance allowlist under `.codencer/connector/config.json`.
The connector also persists a local `.codencer/connector/status.json` snapshot so operators can inspect session state, last heartbeat, and the currently shared instance set without contacting the relay.
Direct relay lookups for steps, artifacts, and gates now probe only authorized online instances and persist the discovered route, so planner HTTP and MCP flows do not depend on prior observation of those IDs.
Planner evidence retrieval through the relay now covers result, validations, logs, artifact lists, and artifact content.
For the end-to-end self-host flow and operating notes, see [docs/SELF_HOST_REFERENCE.md](docs/SELF_HOST_REFERENCE.md), [docs/CONNECTOR.md](docs/CONNECTOR.md), [docs/RELAY.md](docs/RELAY.md), and [docs/mcp/relay_tools.md](docs/mcp/relay_tools.md).

Daemon discovery and evidence notes:
- `GET /api/v1/instance` now exposes stable repo-local daemon identity plus manifest-backed discovery metadata.
- The daemon writes a repo-local instance manifest under `.codencer/instance.json` on startup and after Antigravity bind changes.
- `PATCH /api/v1/runs/{id}` remains best-effort abort. It returns success only when the active step actually reaches `cancelled`; otherwise Codencer leaves an explicit non-cancelled outcome and returns an error instead of claiming a hard kill.

---

## 🚀 The Canonical "Day 0" Path (Human-First)

The standard sequence for performing an audited tactical task:

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

- **Step-Isolation**: Each step executes in its own git worktree, preventing cross-task interference.
- **Immutable Evidence**: All logs, results, and artifacts are namespaced by Run, Step, and Attempt ID under `.codencer/artifacts/<run-id>/<step-id>/<attempt-id>/`, ensuring full auditability of repeated attempts.
- **Workspace Provisioning**: Automatically prepares attempt worktree environments (copying `.env`, symlinking `node_modules`, running `post_create` hooks). Codencer includes an **optional Grove-compatible subset** for environment preparation; it does not depend on the Grove CLI and is designed to coexist with existing `.groverc.json` or `grove.yaml` files.
  - *Inspiration*: This layer was inspired in part by [Grove](https://github.com/verbaux/grove).
  - *Thanks*: Special thanks to [@verbaux](https://github.com/verbaux) for the conceptual foundation of local workspace preparation.

> **Execution Path Note**: Codencer depends on Git Worktrees for isolating task attempts. Therefore, cloning the repository via `git clone` is the **only supported execution path**. Downloading a ZIP source archive will fail during targeted execution.

---

## ⚡️ Quickstart: Local Setup

Get up and running in simulation mode to verify the orchestrator logic.

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

# OR Start in Real Mode (Requires agent binaries like codex-agent or claude)
# Edit .env: ALL_ADAPTERS_SIMULATION_MODE=0
make start
```

For Claude, Codencer invokes the installed CLI as `claude -p --output-format json`, sends the step prompt on `stdin`, and runs from the isolated attempt workspace as the process `cwd`.

The Claude adapter wrapper path is implemented and test-covered in this repo: prompt shaping, normalization, lifecycle behavior, fake-binary integration, and simulation conformance are exercised, but the repo test suite does not run a live authenticated Claude service call. Treat `/api/v1/compatibility` plus your actual runtime environment as the source of truth for local adapter readiness.

### 3. Run Your First Tactical Task
Submit a task and wait for the bridge to report results. For the full auditing sequence, see the **[Canonical Local Runbook](docs/EXAMPLES.md)**.

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

Codencer ensures that every tactical execution is backed by high-fidelity evidence. Follow the **Canonical Sequence** in `EXAMPLES.md` to audit your task:

1.  **Authoritative Summary**: `step result <UUID>` (Start here).
2.  **Raw Execution Trail**: `step logs <UUID>` (The agent's brain).
3.  **Audit Evidence**: `step artifacts <UUID>` and `step validations <UUID>` (The proof).

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
2. **Direct convenience input**: submit a prompt/goal directly and let the CLI deterministically normalize it into `TaskSpec`.

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

This keeps Codencer sharp and narrow as a bridge rather than a workflow brain.

For a deeper dive into agent installation and advanced configuration, see the **[Environmental Reference Guide](docs/SETUP.md)**.

---

## 🛡 Why Codencer?

Agent-driven coding is non-deterministic. Codencer provides the guardrails:

1. **Workspace Safety**: Agents run in isolated Git Worktrees. Diffs are captured and validated before any commit.
2. **Audit-Proof Ledger**: Every attempt is recorded in a local SQLite database (embedded via CGO) with SHA-256 hashes of all artifacts.
3. **Idempotency**: Interrupted tasks can be resumed or securely analyzed post-crash.
4. **Validation-First**: Tasks only "complete" when your defined validation commands (tests, linters) pass.

---

## ⚠️ Known Limitations (Local/Self-Host Alpha)

Codencer’s v2 path is materially real for local and self-host use, but it is still alpha-grade in a few places:
- **No Planner In Core**: Codencer never decomposes, prioritizes, or decides strategy. The planner still owns those decisions.
- **Best-Effort Abort**: `PATCH /api/v1/runs/{id}` and relay abort flows are honest but not universal hard-kill guarantees. A run is only reported cancelled when the adapter actually stops.
- **Opportunistic Remote Routing**: Relay step, gate, and artifact routing is learned from prior responses. Direct remote lookups can fail until the relay has already seen those IDs.
- **Bounded Artifact Transport**: Connector transport rejects oversized artifact bodies instead of turning the relay into a bulk file tunnel. Large binary transfer is intentionally limited.
- **Static Self-Host Auth**: Planner auth is static bearer-token based, suitable for self-host alpha use but not enterprise IAM.
- **Single-Operator Bias**: The current flow is optimized for local/self-host operators, not multi-tenant hosted service use.
- **No Native Workflow Brain**: Ordered task execution remains wrapper- or planner-driven outside Codencer core.

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

1. **Simulation Mode** (`make start-sim`): Only validates the **Orchestrator**. It tests if the ledger, state machine, and CLI are working. It does **not** test if the agent can actually code.
2. **Real Mode**: Tests the full end-to-end loop with real agents. **Codex-agent** is the primary path exercised in this repo; other adapters are implementation-backed but should still be treated as alpha-grade unless your local runtime proves them ready.

---

## 📖 Documentation

Review the following guides to get started with Codencer.

### ⚡️ User Guidance (Start Here)
- **[Operator Runbook](docs/OPERATOR_RUNBOOK.md)** — The canonical "Day 0" flow for humans.
- **[AI Operator Guide](docs/AI_OPERATOR_GUIDE.md)** — Canonical rules for AI planners and assistants.
- **[CLI Automation Patterns](docs/CLI_AUTOMATION.md)** — Machine-safe JSON mode and sequential loops.
- **[Environmental Reference](docs/SETUP.md)** — Prerequisites, configuration, and daemon management.
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

Codencer is released under the **MIT License**. See the [LICENSE](LICENSE) file for the full text.
