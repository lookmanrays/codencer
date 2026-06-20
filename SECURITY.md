# Security Policy

## Supported Scope

This public repository supports the open-source Codencer Core and self-hostable
Gateway, Relay, MCP, connector, and community cloud-control-plane components.

Do not report production managed-service credentials or hosted Codencer service
incidents through public issues. Use the private security contact path below.

## Reporting A Vulnerability

Please report suspected vulnerabilities privately by emailing:

```text
security@codencer.dev
```

Include:

- affected component or command;
- reproduction steps;
- impact and expected severity;
- logs or proof with secrets redacted;
- whether the issue affects local-only, self-host, Gateway/Relay/MCP, or
  cloud-control-plane code.

Do not include real tokens, private keys, OAuth client secrets, Relay planner
tokens, connector private keys, or private repository paths in public reports.

## Public Boundary

The public repository must not contain:

- production Codencer Cloud deployment credentials;
- official connector secrets;
- OAuth provider secrets;
- KMS/Vault/billing/provider credentials;
- committed non-example `.env` files;
- `$CODENCER_HOME` runtime state, sessions, logs, artifacts, or SQLite stores.

See [Public/Private Boundary](docs/architecture/public-private-boundary.md).
