# Public Self-host Release Gate

This specification defines the public OSS/self-host release gate for Codencer.
The current verdict is `NO-GO` until every required gate is proven by current
repository code, release-like artifacts, and deterministic verifiers.

Allowed final verdicts are exactly:

- `GO`
- `NO-GO`

Forbidden verdicts include `PARTIAL`, `mostly GO`, `probably`, `good enough`,
and any caveated acceptance phrase.

## Product Boundary

Codencer is a local/self-host bridge and control plane between AI planners and
coding executors. Codencer is not the planner and not the executor.

The public repository release includes:

- CLI;
- local daemon;
- local connector;
- self-host Relay;
- self-host Gateway;
- public Gateway Console;
- MCP tools and protocol surfaces;
- self-host deployment and operator runbooks;
- release artifact packaging;
- deterministic public self-host verification.

The public repository must not include private managed-service behavior:

- production `mcp.codencer.dev`, `relay.codencer.dev`, or `app.codencer.dev`
  deployment config;
- production provider login;
- billing, quotas, marketplace, or managed runner code;
- private credentials, official connector secrets, KMS/Vault integration, or
  commercial service defaults.

Future Codencer Gateway/Cloud may exist as a separate managed service. Public
self-host release docs may describe that boundary, but public code must default
to local/self-host behavior.

## Definition of GO

The public self-host release is `GO` only when all of these are true:

- a fresh release artifact is built;
- the artifact is unpacked into a clean install directory;
- verifiers use only unpacked artifact binaries for production-like checks;
- a clean `CODENCER_HOME` is used;
- self-host Gateway, Relay, Connector, and daemon start successfully;
- Gateway Console starts in live mode without silent demo fallback;
- project registration/share works;
- machine, connector, relay, and project appear online;
- collection API fields return arrays, never `null`;
- MCP initialize/tools/list/list_projects work against Gateway;
- simple task submit works through Gateway MCP;
- `get_run_report` works for the submitted run ID;
- run lifecycle audit events are emitted and rendered;
- Console project submit works in live mode;
- run result and run history show real report content;
- no MCP/API/UI/CLI output leaks local absolute paths, tokens, private keys,
  daemon URLs, repo roots, environment values, or sensitive local files;
- real executor gates for Codex and Claude Code are proven;
- Antigravity is optional/deferred for this release and must not be claimed as
  proven unless its explicit gate is configured and passes;
- public/private/cloud boundary checks pass;
- docs and acceptance manifests match this release model;
- diff checks pass.

## Required Top-level Gate

`make verify-public-selfhost-rc` is the authoritative release-candidate gate.
It must produce:

- `reports/public-selfhost-rc/<timestamp>/summary.json`
- `reports/public-selfhost-rc/<timestamp>/summary.md`

The report must use exactly one final verdict: `GO` or `NO-GO`.

The gate must fail for:

- fake-only success;
- simulation satisfying a real executor gate;
- missing reports or audit;
- missing pagination/filter coverage where required;
- path/secret leaks;
- unproven Codex or Claude Code;
- unproven Antigravity only when Antigravity is explicitly included in the
  required real executor set;
- mixed or caveated verdict wording.

## Required Evidence

The release gate must include evidence for:

- artifact-backed install path;
- local-first CLI behavior;
- async run lifecycle;
- human interrupt lifecycle;
- real executor proofs;
- MCP protocol proof;
- Gateway Console live proof;
- run history and audit proof;
- redaction proof;
- public/private boundary proof.
