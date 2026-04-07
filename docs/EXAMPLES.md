# Workspace Provisioning: Common Examples

This guide provides configuration templates for common project types to ensure your isolated worktrees are ready for agents.

## Table of Contents
1. [Node.js / TypeScript](#nodejs--typescript)
2. [Advanced: Using Local Provisioning](#advanced-using-local-provisioning)
3. [Broker-Backed Execution](#broker-backed-execution)
4. [OpenClaw ACPX (Experimental)](#openclaw-acpx-experimental)
5. [Audit Walkthrough: Inspecting Provisioning & Broker Context](#audit-walkthrough-inspecting-provisioning--broker-context)

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
./bin/orchestratorctl submit my-first-run examples/tasks/bug_fix.yaml --wait --json
```

## 1.3 Direct Convenience Input

Use direct input when a shell wrapper or planner needs a narrow, automation-friendly submit path without authoring YAML for every task.

Exactly one primary source is required:
- positional task file
- `--task-json <path|->`
- `--prompt-file <path>`
- `--goal <text>`
- `--stdin`

Direct metadata flags are only supported with `--prompt-file`, `--goal`, and `--stdin`:
- `--title`
- `--context`
- `--adapter`
- `--timeout`
- `--policy`
- repeated `--acceptance`
- repeated `--validation`

### Positional TaskSpec File (YAML or JSON)
```bash
./bin/orchestratorctl submit my-first-run examples/tasks/bug_fix.yaml --wait --json
./bin/orchestratorctl submit my-first-run .codencer/task.json --wait --json
```

### Explicit JSON Task Mode
```bash
./bin/orchestratorctl submit my-first-run --task-json .codencer/task.json --wait --json

cat .codencer/task.json | ./bin/orchestratorctl submit my-first-run --task-json - --wait --json
```

### JSON String via Stdin (Pipe)
Ideal for machine-to-machine hand-offs:
```bash
echo '{"version":"v1","goal":"Update README"}' | ./bin/orchestratorctl submit my-first-run --task-json - --wait --json
```

### Prompt File Mode
```bash
./bin/orchestratorctl submit my-first-run \
  --prompt-file prompts/fix-tests.md \
  --title "Fix failing tests" \
  --adapter codex \
  --validation "go test ./pkg/foo" \
  --wait --json
```

### Inline Goal Mode
```bash
./bin/orchestratorctl submit my-first-run \
  --goal "Fix the failing tests in pkg/foo without changing unrelated packages" \
  --title "Fix pkg/foo tests" \
  --adapter codex \
  --policy default_safe_refactor \
  --timeout 180 \
  --validation "go test ./pkg/foo" \
  --wait --json
```

  --wait --json
```

### Multiline Text via Stdin (Heredoc)
Ideal for large, human-readable prompts without creating a file:
```bash
cat <<'EOF' | ./bin/orchestratorctl submit my-first-run --stdin --title "Fix Lints" --adapter codex --wait --json
Fix all lint errors in the internal/app package. 
Exclude the test files. 
Use the 'go-lint' tool.
EOF
```

### Broker-Backed Direct Input
Directly target an IDE-bound agent via the Antigravity Broker using convenience flags:
```bash
./bin/orchestratorctl submit my-first-run \
  --goal "Check UI" \
  --adapter antigravity-broker \
  --wait --json
```
### OpenClaw ACPX (Experimental)
Relay tasks to an OpenClaw-compatible agent via the standardized ACP bridge:
```bash
./bin/orchestratorctl submit my-first-run \
  --goal "Refactor the Auth module" \
  --adapter openclaw-acpx \
  --wait --json
```

### Invalid Multi-Source Example
This is intentionally rejected because `submit` accepts exactly one primary input source.

```bash
./bin/orchestratorctl submit my-first-run examples/tasks/bug_fix.yaml --goal "Fix tests"
```

### Normalization and Provenance

Direct input compiles deterministically into the same internal `TaskSpec` used by file-based submission:
- `version` defaults to `v1`
- `run_id` comes from the CLI run ID
- `title` comes from `--title`, otherwise prompt filename basename, otherwise `Direct task`
- `goal` is the exact text submitted
- repeated `--validation` flags become `validation-1`, `validation-2`, and so on

Each attempt preserves:
- `original-input.*`
- `normalized-task.json`

These are written under the attempt artifact root and are visible through normal artifact inspection. `context` and `acceptance` are retained in the normalized payload for auditability, but they are not currently separate executor-driving runtime fields.

## 1.4 Ordered Sequential Execution (Official v1)

Codencer v1 does not include a native workflow engine or manifest runner. The official sequential model is an external wrapper loop that submits one item at a time with `submit --wait --json`.

Official examples live in `examples/automation/`:
- `run_tasks.sh`
- `run_tasks.ps1`
- `run_tasks.py`

### Bash / zsh wrapper
```bash
examples/automation/run_tasks.sh \
  --run-id run-seq-01 \
  --project codencer-demo \
  --input-mode task-file \
  --tasks-file examples/automation/task_files.txt
```

### PowerShell wrapper
```powershell
./examples/automation/run_tasks.ps1 `
  -RunId run-seq-01 `
  -Project codencer-demo `
  -InputMode prompt-file `
  -TasksFile examples/automation/prompt_files.txt `
  -Adapter codex
```

### Python wrapper
```bash
python3 examples/automation/run_tasks.py \
  --run-id run-seq-01 \
  --project codencer-demo \
  --input-mode goal \
  --tasks-file examples/automation/goals.txt \
  --adapter codex \
  --json
```

Default behavior is stop-on-failure. Use the wrapper’s explicit continue mode when you want to keep running after a non-zero task outcome.

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

Binding only establishes the repo-scoped Antigravity target. Execution still depends on explicit adapter selection in the submitted TaskSpec.

### 3.3 Execute & Audit
To execute via the broker using direct input:

```bash
# Via command line override (Direct Input only)
./bin/orchestratorctl submit <runID> --goal "Fix UI layout" --adapter antigravity-broker --wait --json
```

To execute using a canonical task file, ensure the file specifies the adapter:
```yaml
# examples/tasks/broker_task.yaml
version: v1
title: "Broker Task"
goal: "Check UI"
adapter_profile: antigravity-broker
```

```bash
# Submit the task file
./bin/orchestratorctl submit <runID> examples/tasks/broker_task.yaml --wait --json
```

Auditing a broker task includes extra **Provenance** metadata:
- **Task ID**: The unique session ID on the broker.
- **Bound Repo**: The stable repository path used for the session.
- **Trajectory**: A `trajectory.json` artifact is automatically collected for deep auditing.

### 3.4 Worktree-Aware Execution (Isolation vs. Identity)

When Codencer runs a task via the broker, it distinguishes between two types of paths:

1.  **Repo Root (Identity)**: The stable, long-lived path to your project. This is used by the broker to find the correct IDE instance to talk to.
2.  **Workspace Root (Execution)**: The isolated worktree path where the actual agent execution happens. 

Codencer automatically forwards the temporary worktree path to the broker. The agent will read/write ONLY within that isolated worktree, while the broker uses the repo root to maintain the session.

**Example Inspection:**
In the result metadata, you will see both:
- `broker_repo_root`: Your stable project path.
- `workspace_root`: The specific worktree path for that run.

### Submission via Stdin (Multiline)
Excellent for human operators or AI assistants providing large prompt blocks without creating temporary files.

```bash
cat <<EOF | ./bin/orchestratorctl submit my-run --stdin --title "Update README" --adapter codex --wait --json
Please update the README.md file to include the latest version number (v0.1.0-beta) 
and ensure all links to the Operator Runbook are correct.
EOF
```

### 4. Submission via JSON String (Piped)
Used by automated planners that generate structured task objects.

```bash
echo '{"version":"v1","goal":"Fix typos in main.go","title":"Typos Fix"}' | \
  ./bin/orchestratorctl submit my-run --task-json - --wait --json
```

---

## 🛠 Advanced Developer Flows

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
