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
- **Bridge Report**: The agent exceeded the `timeout_seconds` limit.
- **Audit**: Run `./bin/orchestratorctl step logs <id>` to find the hang.
- **Operator Decision**: Increase timeout in YAML OR simplify the task if the agent is stuck in a loop.

### 3.2 `failed_terminal`
- **Bridge Report**: The task finished, but validations (tests/lint) failed.
- **Audit**: Run `./bin/orchestratorctl step validations <id>` for the failure list.
- **Operator Decision**: Correct the `task.yaml` instructions OR fix the agent's logic manually in the worktree.

### 3.3 `needs_manual_attention`
- **Bridge Report**: An unexpected error or crash occurred.
- **Audit**: Check `.codencer/daemon.log` and `./bin/orchestratorctl step logs <id>`.
- **Operator Decision**: Resolve the system conflict (lock, disk, etc.) and resubmit.

### 3.4 `failed_retryable`
- **Bridge Report**: Transient error (e.g. 429 Rate Limit).
- **Operator Decision**: Wait and resubmit: `./bin/orchestratorctl submit <runID> <file> --wait`.

### 3.5 `cancelled`
- **Bridge Report**: Execution was stopped via `run abort` or CLI interruption.
- **Operator Decision**: Start a new run or step if the original goal is still required.

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
