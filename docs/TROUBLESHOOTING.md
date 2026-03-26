# Troubleshooting Guide

This guide helps you resolve common issues encountered when running the Codencer Orchestration Bridge locally.

## 1. Quick Diagnosis

Always start here to verify your environment:
```bash
# Checks if .codencer/ is setup and if agent binaries are accessible
./bin/orchestratorctl doctor
```

---

## 2. Common Failure Modes

### 2.1 "Connection Refused" (CLI cannot reach Daemon)
**Symptoms**: `./bin/orchestratorctl` returns `error connecting to orchestratord`.
- **Cause**: The `orchestratord` process is not running or is on a different port.
- **Fix**: 
  - Ensure the daemon is started: `./bin/orchestratord`.
  - Check the port in your CLI command or `config/default.json` (default is 8080).

### 2.2 "Agent Binary Not Found"
**Symptoms**: Submitting a task fails immediately; `doctor` shows `MISSING` for an adapter.
- **Cause**: The bridge cannot find the `codex-agent`, `claude-code`, or `aider` binary in your `$PATH`.
- **Fix**:
  - Export the specific path: `export CODEX_BINARY=/path/to/codex-agent`.
  - Or ensure the binary is in your global `$PATH`.

### 2.3 Why are my logs empty? (Simulation Confusion)
**Symptoms**: `./bin/orchestratorctl step logs <id>` shows `No logs available`.
- **Cause**: You are running in **Simulation Mode** (`ALL_ADAPTERS_SIMULATION_MODE=1`).
- **Fix**: Simulation mode stubs agent execution; it does not produce real `stdout.log` or `unified.diff` files. Switch to real mode by unsetting the variable and ensuring agent binaries are installed.

---

### 3.1 `timeout`
- **What it means**: The agent exceeded the `timeout_seconds` defined in your `task.yaml`. The bridge killed the process to prevent hanging.
- **Troubleshoot**: 
  - Check `./bin/orchestratorctl step logs <id>` to see where it got stuck. 
  - If the agent was just slow, increase `timeout_seconds` in the task YAML.
  - If the agent is unresponsive, the bridge may have killed it; check `.codencer/daemon.log`.

### 3.2 `failed_terminal`
- **What it means**: The task finished, but a critical validation (test/lint) failed OR the agent explicitly reported failure.
- **Troubleshoot**:
  - Check `./bin/orchestratorctl step logs <id>` to see where it got stuck. 
  - Run `./bin/orchestratorctl step validations <id>` to see which specific check failed.
  - Review the agent's reasoning in `./bin/orchestratorctl step result <id>`.

### 3.3 `needs_manual_attention` (Ambiguity/Crash)
- **What it means**: The bridge cannot determine the outcome (e.g., the agent crashed without a result JSON, or there's a file conflict in the worktree).
- **Troubleshoot**:
  - Review `./bin/orchestratorctl step logs <id>` for internal agent errors.
  - Check `.codencer/daemon.log` for bridge-side errors.
  - You may need to manually inspect `.codencer/workspace/<runID>/` (if the worktree wasn't cleaned).

### 3.4 `failed_retryable`
- **What it means**: Transient failure (network timeout, rate limit) that can be cleared by trying again.
- **Recovery**: 
  - Review `./bin/orchestratorctl step logs <id>` for internal agent errors.
  - Double-check your API keys in `.env`.
  - Wait a few seconds then run `./bin/orchestratorctl submit <runID> <task.yaml> --wait`.

---

## 4. Resetting Your Environment

If the local state becomes corrupted or you want a fresh start:
```bash
# DANGER: This deletes the database and all artifact history
make nuke

# Rebuild and start fresh
make build
./bin/orchestratord
```
