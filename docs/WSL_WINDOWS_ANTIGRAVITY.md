# WSL, Windows, and Antigravity Topology

This document describes the practical v2 operator topology for Codencer when repos and execution live in WSL/Linux while an IDE or Antigravity broker may live on Windows.

## Recommended Default Layout

- **WSL/Linux**
  - Codencer daemon
  - Git clone
  - worktrees
  - artifacts
  - connector
  - local executor binaries and local adapter execution
- **Windows**
  - IDE
  - Antigravity broker and IDE-side companion process, if used
- **Anywhere reachable by the operator**
  - relay

The default recommendation is simple:
- keep the daemon and connector on the same side as the repo
- keep execution and artifacts local to that side
- treat the relay as remote control plane only

## Trust Boundaries

The trust model is intentionally narrow:

- **Planner**
  - decides what to do
  - calls relay HTTP API or relay MCP
  - does not get raw shell or arbitrary file access from Codencer
- **Relay**
  - authenticates planners and connectors
  - routes requests to the correct shared instance
  - records audit events
  - exposes the canonical remote MCP surface at `/mcp`
  - does not execute code and does not plan
- **Connector**
  - opens outbound websocket session to relay
  - advertises only explicitly shared instances
  - proxies only an allowlisted local daemon API surface
  - does not expose a general tunnel
- **Daemon**
  - owns run, step, gate, artifact, and state-machine truth
  - executes work locally through adapters
  - exposes `/mcp/call` only as a local compatibility/admin bridge
  - must not be exposed directly to the internet
- **Antigravity broker**
  - is separate from relay
  - is optional
  - serves IDE-side discovery/binding concerns, not remote planner control

## Practical WSL / Windows Model

When using WSL and Windows together:

1. Keep the repository checkout in WSL.
2. Run `orchestratord` in WSL with that repo as `--repo-root`.
3. Run `codencer-connectord` in WSL so it can talk to the daemon over local loopback without crossing trust domains.
4. Run the relay wherever you want to terminate remote planner auth.
5. If you use Antigravity, keep the broker on the Windows/IDE side and bind from the daemon when needed.

This avoids the most common problems:
- daemon exposed beyond loopback
- artifacts split across Windows and WSL unexpectedly
- connector proxying through a cross-side filesystem layout it does not control
- relay being mistaken for an execution host

## Loopback and Cross-Side Cautions

- Shared loopback between WSL and Windows can work, but it is an operator convenience, not a new trust boundary.
- Do not assume artifact paths are meaningful off the daemon side. Use result, validation, and artifact APIs rather than raw paths.
- Do not run the connector on Windows while the daemon and repo live in WSL unless you intentionally want a cross-side local hop and understand the failure modes.
- Do not expose the daemon directly just because the relay exists. The relay should be the public remote surface.

## Antigravity Guidance

Antigravity remains local-side execution metadata and binding infrastructure:
- use it when a repo needs to target a live IDE-side agent context
- do not treat it as the relay
- do not treat it as a planner
- do not assume it widens the safe remote surface

The broker and relay are different things:
- **broker**: local/cross-side IDE bridge
- **relay**: authenticated remote control plane for planner calls

## Operator Checklist

- daemon and repo on the same side
- connector on the same side as the daemon
- relay exposed instead of the daemon
- only explicitly shared instances advertised
- Antigravity broker kept separate from relay concerns
- results, validations, and artifacts inspected through APIs and CLI, not raw cross-side paths
