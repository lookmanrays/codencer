# Public Self-Host Release Acceptance

The public self-host release is ready only when a clean operator can deploy
Codencer, connect MCP through Gateway, execute a task through the
connector/daemon path, and retrieve a real run report.

## Required Gate

```bash
make verify-public-selfhost-release
```

This gate must pass in an isolated environment. It uses temporary
`CODENCER_HOME` directories, temporary repos, random local ports, generated
tokens, and release-like binaries from `bin/`.

## Acceptance Evidence

The gate proves:

- public/self-built defaults are self-host/local;
- config precedence is `CLI flags > env vars > user config profile >
  build-time defaults > self-host defaults`;
- Codex, Claude Code, and ChatGPT setup artifacts point to Gateway, not Relay;
- release snapshot tooling runs;
- self-host Gateway and Relay start;
- local daemon and connector participate;
- project sharing reaches Relay/Gateway;
- MCP initialize and tools/list succeed through Gateway;
- project listing returns the shared project;
- manifest execution returns a run id;
- run report retrieval returns that run;
- multi-machine ambiguity returns a structured blocker;
- token and absolute local path leakage checks pass;
- Gateway Console live verification passes.

## Not Verified By This Gate

- ChatGPT product UI;
- Claude Code product client;
- Codex product client;
- public internet exposure of Gateway Console;
- future official managed Codencer service behavior.

Those require separate operator-run evidence.
