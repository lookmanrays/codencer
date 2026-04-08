# ADR 004: OpenClaw + ACPX Integration Strategy

## Status: Decided (Experimental Alpha-Tier)
## Decided by: principal engineer

## Context
Codencer needs a robust integration path for the OpenClaw agent ecosystem. OpenClaw provides a proactive, persistent AI agent platform. To maintain Codencer's "Bridge, Not Brain" doctrine, we require a standardized, structured communication layer that avoids fragile PTY scraping or complex internal orchestration of external agent runtimes.

## Decision
Codencer will implement the **`openclaw-acpx` adapter**, using the **Agent Client Protocol (ACP)** via the **`acpx` CLI** as the integration boundary. 

### Key Objectives
1. **Structured Handoff**: Use `acpx` as a headless, scriptable bridge to OpenClaw-compatible agent sessions.
2. **Session Parity**: Map Codencer `AttemptID` 1-to-1 with `acpx` sessions (using `acpx --session <id>`).
3. **Evidence-First**: Capture `acpx` structured outputs and session logs as native Codencer artifacts.
4. **Experimental Isolation**: Mark the adapter as `Experimental` initially, requiring explicit opt-in via adapter selection.

## Integration Boundary: `acpx`
We choose `acpx` (Option A) over a direct CLI invocation of the OpenClaw runtime (Option B) or a custom broker (Option C) because:
- It uses the standardized **ACP (Agent Client Protocol)**, allowing compatibility with any ACP-compliant agent.
- It handles persistent session state and conversational context natively.
- it provides machine-readable status/result polling without PTY manipulation.

## Technical Mapping

| Codencer Lifecycle | `acpx` command (Conceptual) | Notes |
| :--- | :--- | :--- |
| **Start** | `acpx prompt --session <AttemptID> "<Goal>"` | Run in background via `InvokeLocal`. |
| **Poll** | `acpx status --session <AttemptID> --json` | Check for completion/state. |
| **Cancel** | `acpx stop --session <AttemptID>` | Graceful termination of the agent session. |
| **Collect** | `cat .acp/sessions/<AttemptID>/*` | Ingest session logs/diffs as artifacts. |
| **Normalize** | Parse `acpx` JSON result | Map ACP states to `domain.StepState`. |

## Semantic Mapping
- **`repo_root`**: The base directory of the project clone.
- **`workspace_root`**: The tactical execution directory (worktree) where `acpx` is invoked.
- **`adapter_id`**: `openclaw-acpx`.

## Operational Status: Experimental (Alpha)
The `openclaw-acpx` adapter is implemented and operational in an experimental capacity.
- **Binary**: `acpx` CLI required.
- **Environment**: `OPENCLAW_ACPX_BINARY` for custom pathing.
- **Lifecycle**: Hardened background process tracking with `acpx stop` for robust cancellation.

---

## Rejected Alternatives

### Option B: `openclaw-cli` (Direct Scraped CLI)
**Rejected**: Scrapes raw terminal output. Extremely fragile across different OpenClaw versions and agent configurations. Does not provide structured session management.

### Option C: Custom Broker Bridge
**Rejected**: Adds unnecessary complexity. `acpx` already provides the necessary session brokering locally. A custom broker only makes sense if the agent runtime is forced to reside in a separate restricted environment not reachable via local CLI.
