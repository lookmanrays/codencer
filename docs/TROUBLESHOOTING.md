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
  - Check the port in your CLI command or `.env` file (default is `8085`).

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
- **Bridge State**: The agent exceeded the `timeout_seconds` limit (Bridge killed the process).
- **Audit Truth**: Run `./bin/orchestratorctl step result <id>` for the terminal summary.
- **Evidence Drill-down**: Run `./bin/orchestratorctl step logs <id>` to find the hang.
- **Recovery Decision**: Increase `timeout_seconds` in your TaskSpec YAML OR simplify the instructions. Resubmit to the same mission.

### 3.2 `failed_terminal`
- **Bridge State**: Action finished but goal was not met (e.g., test/lint failure). This is a legacy fallback state.
- **Audit Truth**: Run `./bin/orchestratorctl step state <id>` for the `Reason`.
- **Recovery Decision**: Correct the `task.yaml` instructions OR fix the local project environment. Resubmit to the same mission.

### 3.3 `failed_validation`
- **Bridge State**: Agent finished successfully (exited 0), but the post-execution validations (tests, linting) failed.
- **Audit Truth**: Run `./bin/orchestratorctl step result <id>` to see which validations failed.
- **Recovery Decision**: Provide more specific instructions to the agent or fix the underlying code issue.

### 3.4 `failed_adapter`
- **Bridge State**: The agent binary or process itself failed (e.g. crashed, exited non-zero, or had an internal error).
- **Audit Truth**: Run `./bin/orchestratorctl step logs <id>` to see the agent's stderr.
- **Recovery Decision**: Check your agent configuration, API keys, or binary permissions.

### 3.5 `failed_bridge`
- **Bridge State**: Codencer itself encountered a blocking error (e.g. git worktree conflict, disk full, or lock issue).
- **Audit Truth**: Check the `Reason` in `./bin/orchestratorctl step state <id>`.
- **Recovery Decision**: Resolve the local environment conflict and resubmit.

### 3.3 `needs_manual_attention`
- **Bridge State**: System ambiguity, agent crash, or ambient environment failure. 
- **Audit Truth**: Run `./bin/orchestratorctl step result <id>` for system error logs.
- **Evidence Drill-down**: Check `.codencer/smoke_daemon.log` and `./bin/orchestratorctl step logs <id>`.
- **Recovery Decision**: Resolve the ambient conflict (lock, disk, permissions) and resubmit.

### 3.4 `failed_retryable`
- **Bridge State**: Transient external error (e.g., 429 Rate Limit, Network Drop).
- **Recovery Decision**: Wait for the cooldown and resubmit the mission: `./bin/orchestratorctl submit <runID> <file> --wait`.

### 3.5 `cancelled`
- **Bridge State**: Mission was explicitly aborted by the operator or CLI signal.
- **Recovery Decision**: Start a new mission or step handle if the original goal is still required.

---

### 4.1 Multi-Instance Port Conflict
**Symptoms**: `make start` or `make start-sim` fails with a "failed to start" error.
- **Cause**: Another Codencer instance (or another service) is already using the configured `PORT` (default `8085`).
- **Fix**: 
  - Use `orchestratorctl instance` to see which repository is currently served on the default port.
  - To run another instance, specify a different port in your `.env` or as an environment variable: `PORT=8086 make start`.
  - Ensure your CLI commands also use the correct port: `PORT=8086 orchestratorctl instance`.

---

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

---

## 5. Antigravity Specifics

### 5.1 No instances discovered
**Symptoms**: `antigravity list` returns an empty list.
- **Cause**: Codencer and Antigravity are on different OS sides (e.g. WSL vs Windows).
- **Fix**: Run both Codencer and the Antigravity-supported IDE on the same side.

### 5.2 Binding shows "STALE"
**Symptoms**: `antigravity status` reports `STALE (Process not reachable)`.
- **Cause**: The bound PID no longer exists or the LS instance has crashed/restarted on a different port.
- **Fix**: Re-discover with `antigravity list` and re-bind with `antigravity bind <NEW_PID>`.

### 5.3 Task fails with "Antigravity transport failure"
**Symptoms**: Step state is `failed_adapter` with an RPC error.
- **Cause**: Network interruption or invalid CSRF token.
- **Fix**: Verify the instance is still active in the IDE and re-bind if necessary.
