# Codencer Orchestration Bridge (MVP)

Codencer is a local-first orchestration daemon designed to securely manage, execute, validate, and audit coding tasks performed by autonomous agents. It acts as the system of record between the Planner (MCP clients or LLMs) and the underlying Adapters (Codex, Claude, Qwen).

## Core Architecture
- **Orchestratord (Daemon)**: The persistent state engine using a local SQLite ledger to safely track Runs, Phases, Steps, and Attempts.
- **Adapters**: Abstractions over vendor agents (e.g. Codex) standardizing initialization, polling, capability discovery, and artifact collection.
- **Policy Engine**: Ensures execution safety by intercepting workflows based on heuristic thresholds (e.g., changed files, validation failures) and pausing execution until a human operator responds via a **Gate**.
- **CLI & MCP**: Primary control surfaces. `orchestratorctl` enables terminal-centric operations, while the MCP server provides integration hooks for planning agents.

## Why Codencer?
Agents are chaotic and non-deterministic. Codencer wraps them in a deterministic framework:
1. **Safety**: Agents run in configurable bounds (optional git worktrees, strict diff capturing).
2. **Idempotency**: Runs and attempts are carefully ledgered; interrupted tasks can be resumed or securely analyzed post-crash.
3. **Traceability**: All outputs (stdout, result.json, diffs) are meticulously persisted with **SHA-256 hashes** and **MIME detection** per-attempt in the audit-proof artifact store.

Codencer is a **bridge**, not a brain. The **Planner** (operator or autonomous agent) defines the intent and logic, while Codencer handles the tactical execution, environment isolation, and high-fidelity evidence reporting.

## Local-First Quickstart

The bridge maintains all local state in a hidden `.codencer/` directory in the project root.

### 1. Simple Startup
```bash
# Initialize environment, build binaries, and start the daemon
make dev
```

### 2. Basic Workflow
In a separate terminal:
```bash
# 1. Start a new run
./bin/orchestratorctl run start my-dev-run my-project --force

# 2. Submit a task (using a YAML payload)
./bin/orchestratorctl submit my-dev-run my-task.yaml

# 3. Wait for the result
./bin/orchestratorctl step wait <step-id>
```

### 3. Verification & Cleanup
```bash
# Verify binary availability and paths
make doctor

# Reset intermediate build files (keeps history)
make clean

# Nuke everything (deletes database and history)
make nuke
```

### State Semantics
The Bridge reports state; the Planner decides the next action.
- **pending**: Work is queued in the ledger.
- **running**: Adapter process is active.
- **completed**: Execution reached a successful terminal state as reported by the adapter.
- **failed**: Execution reached an unsuccessful terminal state (failed_terminal or failed_retryable).
- **timeout**: Execution exceeded defined limits (e.g., `timeout_seconds`) and was killed by the bridge.
- **cancelled**: Execution was explicitly stopped by the planner or operator.
- **needs_manual_attention**: The bridge reports a blocking or review-needed condition that it cannot resolve autonomously.

### Simulation Semantics
Simulation mode is a **development-only** feature used to validate orchestrator state transitions and CI/CD pipelines without incurring LLM costs or requiring local model setup. 
- **Not for Performance**: Simulation results do NOT reflect real adapter performance or accuracy.
- **Explicitly Labeled**: All simulated results are marked with `is_simulation: true`.

## Core Concepts
Codencer uses a hierarchical execution model to track work and telemetry:

- **Run**: An execution session that acts as a container for a project-level objective. It houses Phases and Steps.
- **Phase**: A logical grouping of steps within a Run used to organize complex work into sequential segments.
- **Step**: A specific, atomic execution unit issued by the planner (e.g., "Fix bug X").
- **Attempt**: A single, concrete execution try of a Step. One Step may have multiple attempts (e.g., due to retries or adapter fallbacks).

## The Relay Model

Codencer operates as a **Relay** between two distinct planes:
- **Planner (Control Plane)**: The entity that decides *what* to do (e.g., an LLM, a human, or a complex MCP client). It submits a `TaskSpec`.
- **Bridge (Execution Plane)**: Codencer itself. It performs the work using an **Adapter**, enforces **Policies**, and returns a **ResultSpec**.

The Bridge is intentionally "dumb" regarding planning — it does not decide next steps, it only executes and reports.

## Current State: MVP / Beta

**Phase 1 MVP (Complete):**
The foundational orchestration shell is fully operational. A persistent SQLite ledger, an initial state-machine loop, workspace isolation via Git Worktrees, basic MCP tool mapping, and a scaffolding IDE extension have all been implemented.

**Phase 2 System Hardening (Active):**
While structurally sound, the bridge is transitioning from "simulated correctness" to "native reliability" by:
1. **Dismantling Monoliths**: [RESOLVED] Transitioned `DispatchStep` into discrete, fault-tolerant lifecycle coordinators.
2. **Honest Adapter Contracts**: [RESOLVED] Standardized Codex, Claude, and Qwen adapters via a unified execution core (`internal/adapters/common`). Adapters now explicitly detect missing binaries and separate simulation and real execution.
3. **Retrieval Completeness**: [RESOLVED] Exposing standard API, CLI, and MCP retrieval functions to list artifacts and inspect step validation outputs natively.
4. **Strong Policies**: Removing hardcoded env mocks and forcing the Policy Engine to read explicit execution boundaries.
5. **Interactive Integrations**: [RESOLVED] Converted the VS Code UI into an active Control Plane for Gate resolution and workflow retracing.

**Phase 5 Orchestration & MCP Correctness (Complete):**
The core execution engine has been refined for improved reliability. Key improvements include:
1. **Lifecycle Decomposition**: `RunService.DispatchStep` is now a modular coordinator with clear attempt-loop and environment-setup boundaries.
2. **MCP Identity Correctness**: Resolved critical bugs in Step Retry logic to ensure correct RunID propagation.
3. **Structured MCP Payloads**: All tool outputs now return machine-usable JSON, enabling better automated planning.
4. **Environment Robustness**: Worktree- [x] Implement Codex-specific result normalization and outcome mapping (V1.3.1 Complete) <!-- id: 64 -->
- [x] Align Codex adapter reporting with relay contracts (V1.3.1 Complete) <!-- id: 65 -->
- [x] Finalize Batch V1.3.1 alignment and documentation (V1.3.1 Complete) <!-- id: 66 -->
- [ ] Implement state discovery (run/step listing) <!-- id: 52 -->

**Phase 6 Routing & Benchmark Hardening (Complete):**
Hardened task telemetry and routing behavior for architectural honesty.
1. **Explicit Routing**: [RESOLVED] Renamed and documented routing as a deterministic heuristic fallback chain to avoid over-claiming adaptive intelligence.
2. **Truthful Benchmarks**: [RESOLVED] Implemented `is_simulation` tracking in benchmarks to keep stub performance data separate from real execution telemetry.
3. **Observability**: [RESOLVED] Exposed benchmark history and routing configuration via new REST API (`/api/v1/benchmarks`) and MCP tools.
4. **Deterministic Fallbacks**: [RESOLVED] Enforced clear, auditable fallback paths when primary adapters are unavailable or fail.

**Phase 11 Consistency & Polish (Complete):**
Final consistency pass of the initial roadmap. All internal terminology has been standardized and documentation has been updated for technical honesty as a functional MVP.

## Known Limitations

Codencer is a local orchestration bridge, not an autonomous agent or a cloud-scale fleet manager. Current limitations include:
1. **Local-First Only**: Explicitly designed for local developer toolchains; no built-in support for remote multi-tenant execution.
2. **CLI Wrapper Adapters**: Adapters (Codex, Claude, Qwen) operate as CLI wrappers. They require local binary presence and do not provide deeper process-level introspection beyond what the CLI tool exposes.
3. **Implicit Benchmarking**: Benchmarking currently relies on heuristic scoring from result summaries and duration; deeper semantic evaluation is part of the long-term roadmap.
4. **Interactive Shells**: Persistent, stateful interactive shells within an adapter attempt are currently explicitly unsupported.
5. **Maturity**: This tool is currently in **Beta/MVP** state and should be used as an internal or experimental orchestration sidecar.

> **Note on Adapters:** Codex, Claude, and Qwen are currently integrated as CLI wrappers. They require local binary installation (e.g. `claude-code`) unless the corresponding `*_SIMULATION_MODE=1` environment variable is set for testing/evaluation.
>
> **Codex Configuration**:
> - Binary: Expected name is `codex-agent`.
> - Custom Path: Set `CODEX_BINARY=/path/to/binary`.
> - Simulation: Set `CODEX_SIMULATION_MODE=1` to bypass binary checks and use stubs for orchestrator validation.
## Reviewer Summary & Verification

### 1. Verification Commands
The following suite should be run to verify the integrity of the bridge:

```bash
# Build all components
make build

# Run core service and integration tests
make test
```

### 2. Submitting a Task
To submit a canonical task for execution:

```bash
# 1. Start a run (session container)
orchestratorctl run start run-01 my-project

# 2. Submit a task (via YAML file)
orchestratorctl submit run-01 task.yaml
```

### Local Execution Flow
1. **Start a Run**: `orchestratorctl run start my-session my-project`
2. **Submit a Task**: `orchestratorctl submit my-session task.yaml`
3. **Wait for Terminal State**: `orchestratorctl run wait my-session --interval 1s --timeout 5m`

### Discovery and Observability
- **List all runs**: `orchestratorctl run list`
- **List steps in a run**: `orchestratorctl step list <runID>`
- **Inspect step details**: `orchestratorctl step status <stepID>`

The `wait` command will block until the run or step reaches a terminal state (`completed`, `failed`) or requires intervention (`needs_approval`). Progress indicators (.) and status messages are sent to `stderr`, while the final terminal result is printed to `stdout` as a machine-usable JSON object. You can control the polling frequency with `--interval` and enforce client-side safety with `--timeout`.

Example `task.yaml`:
```yaml
version: "1.1"
step_id: "fix-login-01"
phase_id: "execution"
title: "Fix login redirect"
goal: "Update the redirect logic to handle expired tokens"
adapter_profile: "codex"
constraints:
  - "Do not modify the auth provider"
validations:
  - command: "npm test"
    name: "unit-tests"
```

### 3. Inspecting Status & Results (Machine-Readable)
All status and result commands output clean JSON for seamless integration with `jq`:

```bash
# Get run status
orchestratorctl run status run-01 | jq .state

# Get latest step result (works even if in-progress)
orchestratorctl step result step-123 | jq '{state: .state, summary: .summary}'

# Wait for a step to reach a terminal state (blocks and returns JSON)
orchestratorctl step wait step-123 | jq '{state: .state, adapter: .adapter, requested: .requested_adapter, summary: .summary}'
```

> [!TIP]
> Use `.state` to distinguish between `running`, `completed`, `failed_terminal`, and `needs_manual_attention`.

> [!NOTE]
> This is a production-oriented planner-facing CLI surface. Automated polling (`wait` command) and structured results are fully operational for local relay flows.

```bash
# Start the daemon in Orchestration Simulation Mode (verifies state machine only)
make simulate

# In a separate terminal, verify CLI connectivity
./bin/orchestratorctl version
./bin/orchestratorctl run start test-run test-project
```

### 2. Operational Truths
- **Simulation**: The system provides `ALL_ADAPTERS_SIMULATION_MODE=1` to allow end-to-end verification of the orchestration state-machine without requiring local installs of Codex/Claude/Qwen. **NOTE: Simulation does NOT execute real agent logic; it validates the orchestrator's response to stubbed agent outcomes.**
- **Adapters**: Real execution requires the respective CLI binaries to be in the `$PATH`.
- **SQLite Ledger**: All state is local and persistent in `codencer.db` by default.
- **VS Code Extension**: Can be verified by sideloading the `extension/` directory into VS Code. It provides a read/write control plane for the daemon.

### 3. Key Scenarios for Review
- **Gating**: Observe how the system pauses execution when a migration is detected in a simulated attempt.
- **Recovery**: Kill the daemon during a 'running' step and observe how it reconciles the attempt on restart.
- **Auditability**: Use `orchestratorctl step result <id>` to see the full structured JSON evidence of a task.
- **Validation**: See [docs/VALIDATION_SCENARIO.md](docs/VALIDATION_SCENARIO.md) for a repeatable smoke test of the Codex-first execution flow.
