# Executor Adapters and Client Proofs

Fake and simulation adapters are allowed for plumbing smoke tests only. They
must never satisfy the public release gate.

## Required Real Executor Proofs

Public release `GO` requires current proof for:

- Codex CLI;
- Claude Code;
- Antigravity.

If any required real executor proof is missing or unproven, the verdict is
`NO-GO`.

## Simulation Guard

Real executor gates must fail if:

- process or verifier env enables simulation;
- daemon logs contain simulation markers;
- reports contain `is_simulation=true`;
- reports omit required real-execution metadata;
- report/summary/log text contains deterministic simulated output;
- expected adapter/profile does not match the selected executor.

## Executor Profile Model

Executor profiles are user-facing names for adapter/agent/CLI configuration.

Required behavior:

- users can list profiles;
- users can scan/test installed executors;
- projects can define default executor profile;
- tasks can override executor profile;
- dangerous executor profiles require explicit confirmation;
- switching from fake to Codex/Claude/Antigravity must not require project
  re-registration.

## Client Proofs

MCP client configuration must point to Gateway for normal self-host operation.

Required proof/doc status:

- Codex MCP config generated and protocol-smoked;
- Claude Code MCP command/config generated and live-proofed or marked `NO-GO`;
- ChatGPT custom MCP setup documented; product UI proof must not be claimed
  unless actually run;
- direct self-host Relay MCP remains advanced/debug/personal mode.
