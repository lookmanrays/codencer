# Codencer Snippet Library

This document provides specialized configuration and command snippets for advanced Codencer usage. For the primary "Day 0" flow, see the **[Operator Runbook](OPERATOR_RUNBOOK.md)**.

---

## 🏗 Workspace Provisioning (`workspace.json`)

Configure how isolated worktrees are prepared before an agent executes.

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
  --validation "make test-auth" \
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

### OpenClaw ACPX (Experimental / Alpha)
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
