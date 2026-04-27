# DSL and MCP

This document describes the current execution contract and MCP surfaces in Codencer v2.

## TaskSpec

`TaskSpec` is the canonical execution request contract.

It defines:
- goal
- stable identifiers
- adapter preference
- allowed and forbidden paths
- validations
- acceptance criteria
- timeout and simulation intent

Current source of truth:
- `internal/domain/task.go`
- `schemas/task.schema.json`

### Required minimum

A valid task must at least include:
- `version`
- `goal`

The daemon or local CLI may fill omitted `run_id`, `phase_id`, and `step_id` where the route already establishes that context.

## ResultSpec

`ResultSpec` is the normalized execution result contract returned by the daemon and relay-backed flows.

Required truth fields include:
- `version`
- `run_id`
- `step_id`
- `state`
- `summary`

Current source of truth:
- daemon result serialization
- `schemas/result.schema.json`

## Local MCP Surface

The daemon still exposes a local `/mcp/call` compatibility/admin surface.

That surface is useful for local orchestration/admin tooling, but it is not the public remote MCP surface.

## Remote MCP Surface

The relay exposes the public remote MCP surface:
- `/mcp`
- `/mcp/call` compatibility path

Supported MCP methods:
- `initialize`
- `tools/list`
- `tools/call`

Supported relay tools are the `codencer.*` tools documented in [mcp/relay_tools.md](mcp/relay_tools.md).

## MCP Safety Rules

Both local and remote MCP surfaces keep the bridge narrow:
- no raw shell tool exposure
- no unrestricted filesystem browsing
- no bypass of daemon/relay auth rules
- machine-readable errors only

The relay MCP surface also preserves relay auth scopes and connector sharing boundaries.
