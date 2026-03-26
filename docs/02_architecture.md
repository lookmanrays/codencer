> [!NOTE]
> This is a **design specification** and may not fully reflect the current implementation.
> For the latest implementation status, see the [Gap Audit](internal/GAP_AUDIT.md).

# Architecture

## High-level architecture

```text
Planner Chat
   |
   | MCP tools / local CLI
   v
Local MCP Server
   |
   v
Orchestrator Daemon
   |
   +--> Policy Engine
   +--> SQLite Run Ledger
   +--> Artifact Store (filesystem)
   +--> Validation Runner
   +--> Workspace / Git Manager
   +--> Adapter Manager
            |
            +--> Codex Adapter
            +--> Claude Adapter
            +--> Qwen Adapter
            +--> IDE Companion Adapter
            +--> IDE Chat Adapter (later)
```

## Core components

### Orchestrator daemon
System of record.

Responsibilities:
- manage run / phase / step lifecycle
- dispatch steps
- supervise processes
- persist state
- enforce policy
- collect artifacts
- expose state

Recommended implementation:
- Go
- local HTTP or Unix socket
- SQLite
- filesystem artifacts

### MCP server
Thin bridge exposing safe orchestrator operations:
- start run
- start step
- get status
- get result
- approve gate
- reject gate
- retry step
- abort run

### Adapter manager
Provider-neutral execution contract.

Each adapter should support:
- Start
- Poll
- Cancel
- CollectArtifacts
- NormalizeResult
- Capabilities

### Policy engine
Decides:
- continue
- retry
- stop for approval
- fail terminally

Inputs:
- validations
- file changes
- forbidden path touches
- migrations
- dependency changes
- adapter-reported uncertainty
- timeouts

### Run ledger
Persist:
- runs
- phases
- steps
- attempts
- artifacts
- validations
- gates
- audit events

### Artifact store
Deterministic structure like:

```text
.artifacts/
  runs/
    run-0001/
      manifest.json
      phase-execution/
        step-01/
          attempt-01/
            input.json
            stdout.log
            stderr.log
            result.json
            diff.patch
            changed-files.json
            validations.json
```

### Validation runner
Runs:
- lint
- tests
- build
- typecheck
- formatting
- custom commands from policy/task spec

### Workspace / Git manager
Responsibilities:
- detect dirty repo
- allocate isolated worktree when configured
- capture diffs
- cleanup safely
- prevent overlapping writes

## State machine

### Run states
- created
- running
- paused_for_gate
- completed
- failed
- cancelled

### Step states
- pending
- dispatching
- running
- collecting_artifacts
- validating
- completed
- completed_with_warnings
- needs_approval
- failed_retryable
- failed_terminal
- cancelled

## Design rules

### Planner is not source of truth
Planner suggests.
Orchestrator owns actual state.

### Adapter is not control plane
Adapter executes.
Orchestrator decides lifecycle.

### IDE extension is not orchestrator
Extension is only a control/visibility surface.

## Why CLI-first

CLI agents are easier for:
- process supervision
- stdout/stderr capture
- timeouts
- retries
- cancellation
- deterministic wrapping

## Why IDE chat automation is later

IDE AI chats are often implemented inside extension-owned webviews or custom panels.
That makes generic automation brittle.
So:
- CLI path is primary
- IDE companion comes later
- IDE chat adapters are targeted and optional
