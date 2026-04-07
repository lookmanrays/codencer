> [!NOTE]
> This is a **design specification** and may not fully reflect the current implementation.
> For the latest implementation status, see the [Gap Audit](internal/GAP_AUDIT.md).

# Adapters and IDE Plan

## Adapter strategy

The orchestrator should support multiple providers through one common internal contract.

Conceptual interface:
- Name
- Capabilities
- Start
- Poll
- Cancel
- CollectArtifacts
- NormalizeResult

## Adapter order

### 1. Codex first
Why:
- directly solves the immediate pain
- strong local CLI and IDE support
- first-class fit for MVP

### 2. Claude Code second
Why:
- mature terminal-native coding agent
- strong second adapter
- good contrast for adapter-neutral design

### 3. Qwen Code third
Why:
- open/flexible path
- useful hedge against vendor lock-in

### OpenClaw ACPX
- **Status**: 🧪 **Experimental (Operational)**
- **Description**: Standardized ACP (Agent Control Protocol) bridge to the OpenClaw ecosystem.
- **Binary**: `acpx` (configurable via `OPENCLAW_ACPX_BINARY`)
- **Key Capability**: Cross-platform agent communication using a standard protocol interface.

> [!WARNING]
> **OpenClaw (acpx) Support is Experimental**: 
> This adapter is currently in an operational but experimental state. It requires the `acpx` CLI to be installed on the host machine. Lifecycle management (Poll/Cancel) relies on `acpx` session tracking.

## Adapter design rules

- provider quirks stay isolated
- common result schema stays stable
- capability flags live in adapter metadata, not core contracts
- conformance tests required before declaring support

## IDE plan

### Stage 1 — VS Code companion extension
Scope:
- run/phase/step views
- gate actions
- artifact links
- start/retry controls
- local daemon connection

Rule:
- extension is thin
- orchestrator remains source of truth

### Stage 2 — VS Code-like IDE support
Goal:
- package extension for compatible editors
- validate compatibility empirically

### Stage 3 — targeted IDE chat adapter (Proxy-Mediated)
Goal:
- one supported IDE/agent chat bridge only (targeted file-proxy or buffer-mediated)

Priority order:
1. command/API integration
2. extension bridge
3. controlled UI automation as last resort

## Antigravity strategy

Do not assume support.
Validate:
- VS Code extension compatibility
- command execution surface
- webview behavior
- extension install path

Support tiers:
- Tier 0: not supported
- Tier 1: companion extension works
- Tier 2: control features work
- Tier 3: targeted chat adapter works
