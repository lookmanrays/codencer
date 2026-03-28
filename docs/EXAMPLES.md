# Canonical Local Runbook

This is the definitive "Day 0" guide for operating the Codencer orchestration bridge locally.

---

## ⚡️ The 30-Second Mission (Simulation)
Use this flow to verify the bridge logic (ledger, state machine, CLI) without requiring external LLMs or agent binaries.

### 1. Build & Initial Setup
```bash
# Initialize directories, .env, and build binaries
make setup build
```

### 2. Automated Verification (Recommended)
Before manual testing, run the full simulation loop to verify your environment:
```bash
make smoke
```

### 3. Start the Simulated Bridge
If the smoke test passes, you are ready for interactive use:
```bash
# Start in the background (recommended)
make start-sim
```

### 4. Run Your First Tactical Task
Submit a realistic task and wait for the bridge to report results.
```bash
# 1. Start a new orchestration run (System of Record)
./bin/orchestratorctl run start first-run my-project

# 2. Submit a tactical task and wait for completion
./bin/orchestratorctl submit first-run examples/tasks/bug_fix.yaml --wait
# NOTE: The Step ID is a server-generated UUID Handle (e.g., 'step-f027-...') 
# and is printed immediately after submission. Use this UUID Handle for all 
# follow-up 'step' commands.
```

### 4. Inspect the Truth (The Audit Trail)
Once the wait returns, use the Step ID to inspect the high-fidelity evidence captured by the bridge:

```bash
# A. View the human-readable result summary (Authoritative Truth)
./bin/orchestratorctl step result <stepID>

# B. Tail the raw agent logs (What the agent saw/did)
./bin/orchestratorctl step logs <stepID>

# C. List harvested evidence (diffs, artifacts, hashes)
./bin/orchestratorctl step artifacts <stepID>

# D. Verify specific validations (tests/linters)
./bin/orchestratorctl step validations <stepID>

### 💡 Authoritative Truth Sources
- **Immediate Feedback**: `submit --wait` provides the terminal JSON state.
- **Human Summary**: `step result` is the best source for an "at-a-glance" status.
- **Audit Truth**: `step artifacts` and `step validations` are the definitive source for evidence.

---

## 🔍 Visual Audit: What to Expect

### Success Outcome (`completed`)
```text
--- Authoritative Step Result: step-f027-... ---
State:   completed
Summary: Bug fixed successfully. All tests passed.

[GUIDE] Evidence Drill-down:
  Logs:      ./bin/orchestratorctl step logs step-f027-...
  Artifacts: ./bin/orchestratorctl step artifacts step-f027-...
  Validations: ./bin/orchestratorctl step validations step-f027-...
---------------------------
```

### Failure Outcome (`failed_terminal`)
```text
--- Authoritative Step Result: step-e123-... ---
State:   failed_terminal
Summary: Bridge Interface Error: Codex agent finished but one or more tests failed.

[GUIDE] Evidence Drill-down:
  Logs:      ./bin/orchestratorctl step logs step-e123-...
  Artifacts: ./bin/orchestratorctl step artifacts step-e123-...
  Validations: ./bin/orchestratorctl step validations step-e123-...
---------------------------
```
```

---

## 🛠 Flow: Real Codex Hardening
Use this flow for actual daily coding tasks. Requires `codex-agent` installed.

### 1. Configuration
Ensure `.env` has `ALL_ADAPTERS_SIMULATION_MODE=0` and `CODEX_BINARY=codex-agent`.

### 2. Implementation Loop
```bash
# Start the real bridge
./bin/orchestratord

# Submit, Wait, and Audit
./bin/orchestratorctl run start fixer-01
./bin/orchestratorctl submit fixer-01 examples/tasks/bug_fix.yaml --wait
./bin/orchestratorctl step artifacts <stepID>
```

---

## 🏥 Tactical Recovery (What to do on Failure)

The bridge reports what happened; you decide the next move.

### 1. Recovering from `failed_terminal`
*Check validations first.*
1. `step validations <id>`: See which specific test failed.
2. `step logs <id>`: See the agent's internal error messages.
3. **Action**: Correct your `task.yaml` instructions or fix the project environment, then re-submit to the same run.

### 2. Responding to `timeout`
*Check logs first.*
1. `step logs <id>`: Is the agent hanging or just slow?
2. **Action**: If slow, increase `timeout_seconds` in the TaskSpec YAML and re-submit.

### 3. Reconciling `needs_manual_attention`
*Check daemon logs.*
1. View `.codencer/daemon.log` for system-level crashes or adapter errors.
2. **Action**: Restart the daemon or check binary permissions.

---

## 🧹 Maintenance Commands
- **Check Health**: `./bin/orchestratorctl doctor`
- **List History**: `./bin/orchestratorctl run list`
- **Nuke State**: `make nuke` (Deletes ledger and all artifacts)

## 📖 Related References
- [Setup & Environment Guide](SETUP.md) — Prerequisites and custom configuration.
- [Troubleshooting Reference](TROUBLESHOOTING.md) — Advanced error scenarios.
- [Architecture Overview](02_architecture.md) — The "Bridge not Brain" relay model.
