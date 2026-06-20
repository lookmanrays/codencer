# Self-Host MCP Proof

This document describes the deterministic protocol proof for the public
self-host release. It does not claim ChatGPT, Claude Code, or Codex product UI
verification.

## Protocol Checks

`make verify-public-selfhost-release` and `make verify-gateway` use isolated
temporary homes, repos, random ports, self-host Gateway, self-host Relay, local
connector, and local daemon binaries.

The proof performs real MCP JSON-RPC over HTTP through Gateway:

- `initialize`;
- `tools/list`;
- `codencer.list_relays`;
- `codencer.list_projects`;
- `codencer.run_project_manifest`;
- `codencer.get_run_report`;
- ambiguity blocker checks;
- relay unavailable blocker checks;
- token and absolute-path leakage checks.

## Routing Checks

The Gateway proof registers two simulated machines and shared project
locations. A run with `machine_id` succeeds. A run without a machine selector
when multiple locations are available returns structured
`ambiguous_project_location`. A run with multiple Relay profiles and no
`relay_profile_id` returns structured `ambiguous_relay_profile`.

Gateway must never randomly choose among ambiguous project locations.

## Client Config Artifacts

The public self-host verifier generates and validates:

- Codex MCP config pointing to self-host Gateway `/mcp`;
- Claude Code remote HTTP command pointing to self-host Gateway `/mcp`;
- ChatGPT custom MCP setup sheet pointing to self-host Gateway `/mcp`.

If the actual ChatGPT, Claude Code, or Codex product is not run, report those
product-client checks as not verified. Protocol proof is still valid when the
same Gateway MCP endpoint is exercised by the deterministic harness.

## Leakage Rules

MCP output must not contain:

- raw tokens;
- private keys;
- bearer headers;
- connector private identity;
- local daemon URLs;
- absolute local repo paths;
- runtime database or session paths.
