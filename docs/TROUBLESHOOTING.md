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
**Symptoms**: `orchestratorctl` returns `error connecting to orchestratord`.
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
**Symptoms**: `orchestratorctl step logs <id>` shows `No logs available`.
- **Cause**: You are running in **Simulation Mode** (`ALL_ADAPTERS_SIMULATION_MODE=1`).
- **Fix**: Simulation mode stubs agent execution; it does not produce real `stdout.log` or `unified.diff` files. Switch to real mode by unsetting the variable and ensuring agent binaries are installed.

---

## 3. Interpreting Terminal Outcomes

### 3.1 `timeout`
- **What it means**: The agent exceeded the `timeout_seconds` defined in your `task.yaml`. The bridge killed the process to prevent hanging.
- **Action**: Check `orchestratorctl step logs <id>` to see where it got stuck. Increase `timeout_seconds` if the task is simply large.

### 3.2 `needs_manual_attention` (Bridge Interface Error)
- **What it means**: The agent finished its process, but the bridge could not find a valid `result.json` or mandatory artifacts. This often happens if the agent crashed internally or produced malformed output.
- **Action**: 
  - Inspect raw logs: `orchestratorctl step logs <id>`.
  - Check the attempt directory: `ls -R .codencer/artifacts/<runID>/<attemptID>/`.

### 3.3 `failed_retryable`
- **What it means**: A transient failure (e.g. rate limit, temporary disk error) occurred that the bridge identifies as recoverable.
- **Action**: Run `orchestratorctl step retry <stepID>` to attempt the task again.

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
