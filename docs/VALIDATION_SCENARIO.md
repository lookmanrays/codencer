# Local Validation Scenario: Internal Version Bump

This scenario is designed to validate the reliability of the Bridge's execution, evidence harvesting, and reporting flow using a real (non-simulated) Codex-first execution.

## Objective
Update the internal application version string in `internal/app/version.go`.

## 1. Scenario Details

- **Task**: "Update the Version variable in `internal/app/version.go` from its current value to `v0.1.0-alpha`."
- **Adapter**: `codex` (Non-simulated).
- **Target File**: `internal/app/version.go`.
- **Why Safe?**:
  - Modifies a single, low-risk string variable.
  - No side effects on logic or database.
  - Easily reversible via git or manual edit.
- **Why Realistic?**:
  - Mirroring standard release engineering tasks.
  - Validates that the adapter can identify, parse, and modify a Go source file.
  - Exercises the full lifecycle: `dispatch -> run -> harvest -> normalize -> report`.

## 2. Execution Specification (TaskSpec)

This scenario intentionally uses a full `TaskSpec` because it carries richer structure (`constraints`, stable IDs, and explicit adapter choice). Direct convenience input is supported for narrower automation use cases, but it does not replace the full DSL when you need structured task metadata.

```yaml
version: "1.1"
run_id: "validation-run-01"
# [OPTIONAL] step_id: "bump-version-01"
# [OPTIONAL] phase_id: "phase-execution-validation-run-01"
title: "Internal Version Bump"
goal: "Update internal/app/version.go to set Version = \"v0.1.0-alpha\""
adapter_profile: "codex"
constraints:
  - "Do not change the package name"
  - "Only update the Version variable"
```

## 3. Expected Outcomes

### Step Result (terminal check)
- `state`: `completed`
- `summary`: Successful update of version string.

### Harvested Evidence (bridge-reported)
- **Artifacts**:
  - `result.json`: Canonical result spec from Codex.
  - `stdout.log`: Process output (should mention success or logs).
- **Hardened Metadata**:
  - Every artifact should have a **SHA-256 hash**.
  - `stdout.log` should have MIME type `text/plain`.
  - `result.json` should have MIME type `application/json`.
  - `raw_output_ref` must point to the absolute path of `stdout.log`.

## 4. Verification Steps

1. **Automated Submission (Recommended)**:
   Ensure the daemon is running in a separate terminal (`make run`), then execute:
   ```bash
   make validate
   ```
   This will automatically start the run, submit the task, and wait for completion.

2. **Manual Submission**:
   Alternatively, you can run the steps manually:
   ```bash
   ./bin/orchestratorctl run start validation-run-01 validation-project
   ./bin/orchestratorctl submit validation-run-01 docs/validation_task.yaml --wait --json
   ```
   A narrower direct-input equivalent is also possible when you only need goal/title/adapter-style fields:
   ```bash
   ./bin/orchestratorctl submit validation-run-01 \
     --goal "Update internal/app/version.go to set Version = \"v0.1.0-alpha\"" \
     --title "Internal Version Bump" \
     --adapter codex \
     --wait --json
   ```
   Use the full TaskSpec when you need richer fields like `constraints`.
   For ordered multi-task runs, keep the same per-task submit contract and use the official wrapper patterns documented in `docs/CLI_AUTOMATION.md` and `examples/automation/`.
3. **Verify Result**:
   In machine-facing flows, use `--json` so stdout contains a single terminal payload. Inspect the terminal outcome of the `submit --wait --json` command:
   
   Example terminal payload:
   ```json
   {
     "state": "completed",
     "summary": "Updated internal/app/version.go to v0.1.0-alpha",
     "artifacts": {
       "result.json": "/home/lookman/projects/codencer/artifacts/val-step-01-a1/result.json",
       "stdout.log": "/home/lookman/projects/codencer/artifacts/val-step-01-a1/stdout.log"
     },
     "raw_output_ref": "/home/lookman/projects/codencer/artifacts/val-step-01-a1/stdout.log"
   }
   ```

4. **Verify Evidence Metadata**:
   To see the hardened **SHA-256 hashes** and **MIME types**, check the artifacts specifically:
   ```bash
   ./bin/orchestratorctl step artifacts bump-version-01
   ```
   
   Example formatted `artifacts` output:
   ```json
   [
     {
       "id": "art-e3b0c442...",
       "name": "stdout.log",
       "type": "stdout",
       "hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
       "mime_type": "text/plain"
     }
   ]
   ```
