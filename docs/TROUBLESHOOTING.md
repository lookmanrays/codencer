# Troubleshooting Reference

This guide helps you resolve common issues encountered when running the Codencer Orchestration Bridge locally.

## 1. Quick Diagnosis

Always start here to verify your environment:
```bash
# Checks if .codencer/ is setup and if agent binaries are accessible
./bin/orchestratorctl doctor
```

---

## 2. Common Infrastructure Failures

### 2.1 "Connection Refused" (CLI cannot reach Daemon)
**Symptoms**: `./bin/orchestratorctl` returns `error connecting to orchestratord`.
- **Cause**: The `orchestratord` process is not running or is on a different port.
- **Fix**: 
  - Ensure the daemon is started: `make start` or `make start-sim`.
  - Check the port in your CLI command or `.env` file (default is `8085`).

### 2.2 "Agent Binary Not Found" (`failed_adapter`)
**Symptoms**: Submitting a task fails immediately; `doctor` shows `MISSING` for an adapter.
- **Cause**: The bridge cannot find the `codex-agent`, `claude-code`, or `aider` binary in your `$PATH`.
- **Fix**:
  - Export the specific path: `export CODEX_BINARY=/path/to/codex-agent`.
  - Or ensure the binary is in your global `$PATH`.

### 2.3 "Workspace Creation Failed" (`failed_bridge`)
**Symptoms**: `failed_bridge` reported during attempt.
- **Cause**: Git worktree conflict or permission issue.
- **Fix**: Run `git worktree prune` and ensure the `~/.codencer/workspaces` directory is writable.

---

## 3. Interpreting Step States

Every task result includes a `state`. Understanding the difference between **Infrastructure** and **Goal** failure is key to recovery.

### 3.1 Goal & Task Failures (Fix the code/prompt)

- **`failed_terminal`**: The agent finished execution, but the summary reports that the goal was not met.
    - *Resolution*: Refine your task instructions or check if the agent is stuck in a loop.
- **`failed_validation`**: The agent finished and exited with 0, but your post-execution validations (tests, lint) failed.
    - *Resolution*: Review the `step validations` output and fix the code or the test.

### 3.2 Infrastructure & Bridge Failures (Fix the system)

- **`failed_adapter`**: The agent process crashed or exited with a non-zero code before it could report its outcome.
    - *Resolution*: Check `step logs` for agent-side crashes (e.g., API key errors, OOM).
- **`failed_bridge`**: Codencer encountered an internal error during worktree creation or **provisioning**.
    - *Resolution*: Check the `provisioning` telemetry in `step result` for copy/link errors.
- **`timeout`**: The execution exceeded `timeout_seconds` and was killed by the bridge.
    - *Resolution*: Increase the timeout in your TaskSpec or simplify the task.

---

## 4. Antigravity & Broker Issues (Experimental)

### 4.1 "Broker bind error: connection refused"
- **Cause**: The Windows-side `agent-broker.exe` is not running.
- **Fix**: Start the broker on the host machine. Verify port 8088 is open.

### 4.2 "No instances discovered"
- **Cause**: Antigravity is not active in your IDE or the `.gemini` daemon directory is hidden/unreachable.
- **Fix**: Open a project in VS Code with Antigravity enabled. Ensure your WSL mount for `C:` is working correctly.

### 4.3 "Stale Binding"
- **Cause**: The IDE instance was closed or restarted.
- **Fix**: Run `orchestratorctl antigravity list` and re-bind to the new PID.

---

## 5. Resetting the Bridge

If the local state becomes corrupted or you want a fresh start:
```bash
# DANGER: This deletes the database and all artifact history
make nuke

# Rebuild and start fresh
make build
make start-sim
```
