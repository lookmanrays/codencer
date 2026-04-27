# Security and Operations

This document describes the current operational security model for Codencer v2.

## Security Principles

- **Bridge Not Brain**: Codencer executes, waits, records, and reports. It does not plan.
- **Local Execution**: adapters and artifacts stay local to the daemon side.
- **Explicit Sharing**: connector discovery does not imply exposure; config is the allowlist.
- **Narrow Remote Surface**: relay HTTP API and relay MCP expose only instance-scoped orchestration operations.
- **Evidence First**: results, validations, and artifacts are recorded as local truth.

## Remote Surfaces

The only intended remote control surfaces are:
- relay planner API
- relay MCP
- connector outbound websocket

The daemon is not intended to be internet-facing.

## What Is Not Exposed

Codencer v2 does not expose:
- raw shell execution through relay or MCP
- arbitrary filesystem browsing through relay or MCP
- generic network tunneling through the connector
- implicit repo sharing
- unauthenticated remote control

## Local Safety

The daemon preserves local safety by:
- anchoring execution to an explicit repo root
- isolating attempts with worktree and provisioning logic
- persisting run, step, gate, and artifact truth locally
- keeping abort semantics honest when cancellation is not confirmed

## Remote Safety

The relay and connector preserve remote safety by:
- authenticating planners with bearer tokens
- authenticating connectors with enrollment plus signed challenge/response
- allowing only explicitly shared instances to be advertised
- routing only through the connector allowlist
- persisting audit events for remote control actions

## Current Honest Limitations

- planner auth is static-token based
- relay resource routing for `step`, `gate`, and `artifact` ids depends on authorized online shared instances being reachable; the relay now probes for missing route hints and persists successful matches, but it still fails closed on offline or ambiguous matches
- large artifact transfer is intentionally bounded
- abort remains best-effort unless the adapter actually stops
- current self-host auth model is alpha-grade, not enterprise IAM

## Operator Guidance

- keep the daemon on loopback or another trusted local boundary
- expose the relay instead of exposing the daemon
- keep the connector on the same side as the daemon when possible
- inspect results, validations, and artifacts via CLI or API, not raw path assumptions
- treat Antigravity broker and relay as separate trust domains
