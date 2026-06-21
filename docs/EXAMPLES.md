# Codencer Snippet Library

This document provides specialized and legacy configuration snippets for advanced Codencer usage. For the current v0.3 local/self-host RC flow, start with [Local Quickstart](quickstart-local.md), [Self-Host Relay Quickstart](quickstart-self-host-relay.md), and [MCP Integrations](mcp/integrations.md).

---

## 🏗 Workspace Provisioning (`workspace.json`)

Configure how isolated worktrees are prepared before an agent executes.

Codencer is Grove-compatible. Codencer can read a safe subset of `grove.yaml`
and `.groverc.json`. It uses those files only when native
`.codencer/workspace.json` is absent or incomplete. Native
`.codencer/workspace.json` has precedence. Codencer does not depend on the Grove
CLI. Codencer does not write Grove state files.

The native file lives at `.codencer/workspace.json` and is opt-in; `codencer
project init` still creates only `.codencer/project.json` by default. Grove
compatibility is read-only fallback mapping for provisioning fields:

- `grove.yaml`: `workspace.setup.copy`, `workspace.setup.symlinks`, and
  `workspace.hooks.post_create`.
- `.groverc.json`: `symlink` and `afterCreate`.

### Node.js / TypeScript
Efficiently share `node_modules` avoiding costly file copies.
```json
{
  "provisioning": {
    "copy": [".env", ".env.local"],
    "symlinks": ["node_modules"],
    "hooks": {
      "post_create": "npm install --prefer-offline"
    }
  }
}
```

### Go / Modules
```json
{
  "provisioning": {
    "copy": [".env"],
    "symlinks": ["vendor"],
    "hooks": {
      "post_create": "go mod download"
    }
  }
}
```

### Python / Pipenv
```json
{
  "provisioning": {
    "copy": [".env"],
    "symlinks": [".venv"],
    "hooks": {
      "post_create": "pipenv install --deploy --ignore-pipfile"
    }
  }
}
```

---

## ⚡️ Specialized Submission Flows

### 5.2 Rich Submission with Metadata
Targeting a specific adapter for a task saved in a markdown file.
```bash
./bin/orchestratorctl submit my-run \
  --prompt-file prompts/refactor-auth.md \
  --title "Auth Refactor" \
  --adapter codex \
  --timeout 300 \
  --validation "go test ./internal/service" \
  --acceptance "Login still works" \
  --wait --json
```

### 5.3 Piped Task Definitions
Machine-to-machine handoff without temporary files.
```bash
echo '{"version":"v1","goal":"Fix typos","title":"Small Fix"}' | \
  ./bin/orchestratorctl submit my-run --task-json - --wait --json
```

### 5.4 Multiline Stdin (Heredoc)
```bash
cat <<'EOF' | ./bin/orchestratorctl submit my-run --stdin --title "Fix Lints" --adapter codex --wait --json
Fix all lint errors in the internal/app package. 
Exclude the test files. 
EOF
```

### 5.5 Claude Headless Execution
Use the Claude adapter when the `claude` CLI is installed locally and reachable through `CLAUDE_BINARY` or `$PATH`.
```bash
cat <<'EOF' | ./bin/orchestratorctl submit my-run --stdin --title "Claude Audit" --adapter claude --wait --json
Review the auth package, explain the failing behavior, and propose a minimal fix.
EOF
```

For Claude attempts, the standard evidence set includes:
- `prompt.txt`
- `stdout.log`
- `stderr.log`
- `result.json`

### OpenClaw ACPX (Experimental)
Relay tasks to an OpenClaw-compatible executor via the standardized ACP bridge. Use `--wait --json` for synchronous machine-safe handoffs.
```bash
./bin/orchestratorctl submit my-run \
  --goal "Fix UI layout issues in the landing page" \
  --adapter openclaw-acpx \
  --wait --json
```

### Antigravity Broker (Cross-Side)
Requires a previous `orchestratorctl antigravity bind <PID>`.
```bash
./bin/orchestratorctl submit my-run \
  --goal "Check React component alignment" \
  --adapter antigravity-broker \
  --wait --json
```

---

## 🔍 Auditing & Evidence

### Inspecting Provisioning Telemetry
See exactly how the workspace was prepared.
```bash
./bin/orchestratorctl step result <HANDLE> --json | jq '.provisioning'
```

### Listing Collected Artifacts
```bash
./bin/orchestratorctl step artifacts <HANDLE>
```

### Streaming Raw Logs
```bash
./bin/orchestratorctl step logs <HANDLE>
```

### Inspecting Claude-Specific Evidence
```bash
./bin/orchestratorctl step artifacts <HANDLE>
./bin/orchestratorctl step result <HANDLE> --json | jq '.raw_output_ref, .artifacts["prompt.txt"], .artifacts["stderr.log"]'
```
