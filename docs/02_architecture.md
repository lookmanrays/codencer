# Architecture

This document describes the current Codencer v2 runtime architecture.

## High-Level Model

```text
Planner / Chat
   |
   | Relay HTTP API or Relay MCP
   v
Relay
   |
   | Authenticated outbound websocket
   v
Connector
   |
   | Narrow allowlisted local API proxy
   v
Local Codencer Daemon
   |
   +--> SQLite state and settings
   +--> Artifact store
   +--> Workspace / git manager
   +--> Validation runner
   +--> Adapter dispatch
   +--> Gate and recovery services
```

Execution stays local. Planning stays outside Codencer.

## Core Runtime Roles

### Local daemon

The daemon is the local system of record.

Responsibilities:
- manage run, step, attempt, and gate lifecycle
- persist state
- dispatch adapters
- collect artifacts and validations
- expose local `/api/v1` and local compatibility/admin `/mcp/call`

The daemon is not the public internet-facing MCP server.

### Connector

The connector is the outbound bridge between relay and local daemon.

Responsibilities:
- persist connector identity
- enroll with relay
- authenticate with signed challenge/response
- advertise explicitly shared instances
- proxy only a narrow local API surface

The connector does not plan and does not execute work directly.

### Relay

The relay is the remote control plane.

Responsibilities:
- authenticate planners
- authenticate connectors
- track online connectors and advertised instances
- route planner requests to the correct shared local instance
- persist audit events
- expose relay HTTP API and relay MCP

The relay is not a planner and not an executor.

## Public Surfaces

### Remote/public

- relay HTTP API under `/api/v2`
- relay MCP under `/mcp`
- relay MCP compatibility path `/mcp/call`
- connector websocket under `/ws/connectors`

### Local/private by default

- daemon HTTP API under `/api/v1`
- daemon-local `/mcp/call` compatibility/admin bridge

## State And Evidence

The current authoritative state lives in the daemon:
- runs
- steps
- attempts
- gates
- artifacts
- validations

The relay stores only the remote control-plane state it needs:
- connector identity
- instance advertisement records
- resource routing hints
- audit events
- enrollment/challenge state

## Trust Boundaries

- planner decides what to do
- relay authenticates and routes
- connector limits remote reach to a narrow allowlist
- daemon executes and records truth locally
- adapters do local work only

There is no raw shell or arbitrary filesystem browsing surface in the relay or connector.

## WSL / Windows Model

The practical default is:
- daemon, connector, repo, worktrees, and artifacts in WSL/Linux
- IDE and Antigravity broker on Windows when needed
- relay wherever the operator hosts the remote control plane

See [WSL / Windows / Antigravity Topology](WSL_WINDOWS_ANTIGRAVITY.md) for detailed placement guidance.
