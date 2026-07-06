# Local-first Source of Truth

Codencer remains local-first. Local daemon state, local run reports, local
artifacts, and local operator intent are authoritative for local CLI execution.
Gateway is a control plane, index, and optional sync target. Gateway is not the
global source of truth for every local run.

## Required Behavior

- Manual local CLI runs stay local by default.
- `codencer submit` must work without Gateway, Relay, Connector, or internet
  access when the selected executor can run locally.
- Local reports remain under local Codencer runtime state by default.
- Gateway-submitted runs are indexed by Gateway with safe metadata and
  sanitized report excerpts.
- Local-only runs are not automatically uploaded to Gateway.
- Raw logs, raw artifacts, repo roots, daemon URLs, and local paths are not
  published by default.

## Project and Workspace Config

Committed project config remains limited to commit-safe data such as:

- project identity;
- default adapter/executor profile;
- safe labels;
- repository-scoped settings that do not expose machine-local state.

Local state remains under `CODENCER_HOME`, including:

- machine identity;
- connector identity;
- daemon URLs;
- runtime DBs;
- run reports;
- raw logs/artifacts;
- tokens and credentials.

## Explicit Sync and Publish

Any sync/publish behavior must be explicit.

Required controls:

- dry-run/preview for sync/publish where practical;
- clear destination and scope;
- safe metadata by default;
- opt-in raw artifact/log publishing only if implemented with redaction and
  confirmation;
- audit records for sync/publish actions.

## Redaction Boundary

Default CLI/MCP/UI/Gateway output must not expose:

- absolute local paths;
- repo roots;
- daemon URLs;
- tokens or bearer headers;
- private keys;
- environment values;
- raw artifact paths;
- local runtime DB/report paths.
