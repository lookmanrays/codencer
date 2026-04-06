# Manual Verification Checklist

Follow these steps after a fresh `git clone` and `make setup` to verify that your bridge is ready for tactical execution.

## 1. Build Verification
- [ ] Run `make build`.
- [ ] Verify `bin/orchestratord` and `bin/orchestratorctl` exist.
- [ ] Run `./bin/orchestratorctl doctor` and ensure Git, Go, and CC are **[OK]**.

## 2. Daemon & Connectivity
- [ ] Start the daemon in simulation mode: `make start-sim`.
- [ ] In a new terminal, run `./bin/orchestratorctl instance`.
- [ ] Verify it reports the correct repository path and port (8085).

## 3. Simulation Run (The "Smoke Test")
- [ ] Start a run: `./bin/orchestratorctl run start smoke-test local-verify`.
- [ ] Submit the example task: `./bin/orchestratorctl submit smoke-test examples/tasks/bug_fix.yaml --wait --json`.
- [ ] Verify the state reaches `completed` (in simulation mode).

## 4. Evidence Inspection
- [ ] Run `./bin/orchestratorctl step result <UUID>`.
- [ ] Verify that a `Summary` is present.
- [ ] Run `./bin/orchestratorctl step logs <UUID>`.
- [ ] Verify it says `No logs available` (expected for simulation mode) or shows stub output.
- [ ] Run `./bin/orchestratorctl step artifacts <UUID>`.
- [ ] Verify the artifact directory exists in `.codencer/artifacts/`.

## 5. Antigravity Broker (Optional/Experimental)
- [ ] If using WSL/Windows, start `agent-broker.exe` on Windows.
- [ ] Run `./bin/orchestratorctl antigravity list`.
- [ ] Verify that at least one IDE instance is discovered (or handle 'no instances' gracefully).

---

**Status**: If steps 1-4 pass, your bridge is **Operational (Stable Beta)**.
