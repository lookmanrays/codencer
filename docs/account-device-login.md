# Account Device Login

`codencer login` performs a device-code style Gateway login:

```bash
codencer login --gateway https://mcp.codencer.dev
```

The CLI asks Gateway for a device code, prints the verification URL and user
code, polls for approval, then stores the session in:

```text
$CODENCER_HOME/session.json
```

The session contains Gateway URL, MCP URL, user id, workspace id, scopes,
expiry, and a Gateway bearer token. The token is local user state and is not
written into project `.codencer/` files.

Useful commands:

```bash
codencer whoami --json
codencer logout --json
```

`--json` output reports that a token is configured but does not print the token.
Deterministic local tests may use `--dev-approve`; public/product login proof
must come from the real product flow with saved evidence.
