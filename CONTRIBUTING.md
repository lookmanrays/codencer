# Contributing To Codencer

Thank you for contributing to Codencer Core. Codencer is an open-source
local/self-host bridge for coding-agent execution. It is not a planner, hosted
UI, billing system, or managed Gateway/Cloud service.

## Development Principles

- Preserve the bridge boundary: planner decides, executor works, Codencer records
  state and evidence.
- Keep local-first safety and explicit sharing boundaries.
- Do not add commercial cloud scope, billing, hosted UI, or planner behavior in
  this repository.
- Keep behavior typed, testable, deterministic, and operationally boring.
- Do not claim live ChatGPT, Codex, or Claude proof unless that client was
  actually exercised and evidence was saved.

## Local Setup

Prerequisites:

- Go 1.25+
- Git
- SQLite-capable local environment
- macOS, Linux, or WSL2

Build:

```bash
make build-codencer
```

Initialize local state in an isolated home when testing contributor changes:

```bash
export CODENCER_HOME="$(mktemp -d)"
./bin/codencer init --json
./bin/codencer machine show --json
```

## Verification

For narrow code changes, run the focused package tests and the smallest relevant
smoke target. Before opening or merging release-path changes, run:

```bash
gofmt -w ...
go test ./...
make build-codencer
make verify-project-config
make verify-local-execution
make verify-local-relay-mcp
make verify-runtime-recovery
make verify-live-matrix
make acceptance-local-production
make verify-release
make verify-local-prod
make activation-preflight
make verify-docs-links
git diff --check
```

Use temporary `CODENCER_HOME` directories and temporary repositories for
automation. Do not rely on or mutate a contributor's real local Codencer config
when writing tests.

If optional live checks are not run, report them as skipped or pending. Do not
convert skipped live-product gates into passed gates.

## Pull Requests

1. Create a feature branch from the active integration branch.
2. Keep the diff narrow and aligned with the architecture boundaries.
3. Update docs when behavior changes.
4. Include tests or smoke coverage proportional to the risk.
5. Describe commands run and any skipped live/external checks.

## Contribution Licensing

Codencer Core is licensed under the Apache License, Version 2.0. By submitting a
contribution, you agree that your contribution is licensed under Apache-2.0.

No Contributor License Agreement is required unless the project later documents
one explicitly.

Use Developer Certificate of Origin style signoff when practical:

```text
Signed-off-by: Your Name <you@example.com>
```

By signing off, you certify that you have the right to submit the contribution
under the Apache-2.0 license and that the contribution can be included in
Codencer Core.

## Trademarks

Apache-2.0 covers the open-source software in this repository. It does not grant
rights to use Codencer trademarks, logos, domains, official connector identity,
Codencer Gateway, Codencer Cloud, Official Codencer MCP, or hosted Codencer
services. See [TRADEMARKS.md](TRADEMARKS.md).
