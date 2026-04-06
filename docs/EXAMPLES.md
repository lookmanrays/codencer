# Workspace Provisioning: Common Examples

This guide provides configuration templates for common project types to ensure your isolated worktrees are ready for agents.

## Table of Contents
1. [Node.js / TypeScript](#nodejs--typescript)
2. [Advanced: Using Local Provisioning](#advanced-using-local-provisioning)
3. [Broker-Backed Execution](#broker-backed-execution)
4. [Audit Walkthrough: Inspecting Provisioning & Broker Context](#audit-walkthrough-inspecting-provisioning--broker-context)

## Node.js / TypeScript

Efficiently share your `node_modules` avoiding costly file copies.

```json
{
  "provisioning": {
    "copy": [".env", ".env.local"],
    "symlinks": ["node_modules"],
    "hooks": {
      "post_create": "npm install --offline"
    }
  }
}
```

## Go / Modules

Ensure the vendor directory is present or dependencies are downloaded.

```json
{
  "provisioning": {
    "copy": [".env", "config/secrets.json"],
    "symlinks": ["vendor"],
    "hooks": {
      "post_create": "go mod download"
    }
  }
}
```

## Python / Pipenv

Link your virtual environment and copy your `.env` file.

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

## Grove Compatibility (Zero-Config Merging)

If your project already has a `.groverc.json`, Codencer automatically leverages it:

### `.groverc.json` (Existing)
```json
{
  "symlink": ["node_modules", "dist"],
  "afterCreate": "make setup"
}
```

### Resulting Codencer Spec
- **Copy**: `[]` (None defined in Grove)
- **Symlinks**: `["node_modules", "dist"]`
- **Hooks**: `{ "post_create": "make setup" }`

### Overriding Grove for Codencer
If you need to change only the hook for Codencer, add a native config:

```json
{
  "provisioning": {
    "hooks": {
      "post_create": "make codencer-special-setup"
    }
  }
}
```
**Precedence**: Codencer will now use your native hook but still pull the symlink list from Grove.

---

# Canonical Local Runbook (Day-0 Guide)

This guide provides the definitive operational sequence for running audited tactical tasks with Codencer.

## 1. The Standard Tactical Task (Local)

Use this sequence to run a task with a local agent (e.g., Codex or Simulation Mode).

### 1.1 Start the Mission
```bash
# 1. Start the daemon (Simulation mode for this example)
make start-sim

# 2. Initialize a new Run (System of Record)
./bin/orchestratorctl run start my-first-run my-project
```

### 1.2 Submit & Wait
```bash
# Submit the task and follow it to completion
./bin/orchestratorctl submit my-first-run examples/tasks/bug_fix.yaml --wait
```

---

## 2. The Canonical Audit Sequence

Once a task reaches a terminal state (`completed`, `failed_terminal`), follow this sequence to audit the evidence.

### 2.1 The Authoritative Summary
Start here to see "What" happened and "Why".
```bash
./bin/orchestratorctl step result <UUID>
```

### 2.2 The Workspace Preparation (Provisioning)
Check the `Workspace Provisioning` section in the result output to see which files were copied or symlinked.

### 2.3 The Raw Execution Trail
Check the agent's "thoughts" and raw stdout.
```bash
./bin/orchestratorctl step logs <UUID>
```

### 2.4 The Proof (Artifacts & Validations)
Inspect the actual diffs and test results.
```bash
./bin/orchestratorctl step artifacts <UUID>
./bin/orchestratorctl step validations <UUID>
```

---

## 3. Broker-Backed Execution (Antigravity)

Use this flow for **cross-side execution** (e.g., Codencer in WSL controlling Antigravity in Windows).

### 3.1 Start the Broker
Ensure the `agent-broker.exe` is running on the Windows side.

### 3.2 Bind the Repository
Connect your local repository clone to an active Antigravity session.
```bash
# 1. List active IDE instances
./bin/orchestratorctl antigravity list

# 2. Bind to an instance
./bin/orchestratorctl antigravity bind <PID>
```

### 3.3 Execute & Audit
Submit as usual. Codencer will automatically route the task through the broker.
```bash
./bin/orchestratorctl submit <runID> <file> --wait
```

Auditing a broker task includes extra **Provenance** metadata:
- **Task ID**: The unique session ID on the broker.
- **Bound Repo**: The stable repository path used for the session.
- **Trajectory**: A `trajectory.json` artifact is automatically collected for deep auditing.

---

## 4. Advanced Provisioning Examples

### Node.js (Symlinked dependencies)
**.codencer/workspace.json**
```json
{
  "provisioning": {
    "copy": [".env"],
    "symlinks": ["node_modules"],
    "hooks": {
      "post_create": "npm install --prefer-offline"
    }
  }
}
```

### Python (Virtual Environment)
**.codencer/workspace.json**
```json
{
  "provisioning": {
    "copy": [".env"],
    "symlinks": ["venv"],
    "hooks": {
      "post_create": "source venv/bin/activate && pip install -r requirements.txt"
    }
  }
}
```

---

## 5. Audit Walkthrough: The "Authoritative Truth"

When a task fails, use the following mental model to debug:

1.  **State=failed_bridge**: Infrastructure issue. Check `doctor` and provisioning logs.
2.  **State=failed_adapter**: Agent crashed. Check `step logs`.
3.  **State=failed_validation**: Agent finished, but tests failed. Check `step validations`.
4.  **State=failed_terminal**: Goal not met. Check `step result` for the summary.
