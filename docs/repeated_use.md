# Repeated-Use & Evidence Trustworthiness

Codencer is built to handle repeated planner loops where the same task may be attempted multiple times. This guide explains how to inspect the distinct evidence generated across these attempts.

## Authoritative Truth

The "authoritative truth" of a step is the result of its **latest** attempt. This is what the bridge reports back to the planner.

Run the following to see the latest summary and state:
```bash
./bin/orchestratorctl step result <step-uuid>
```

If the latest attempt failed due to validations (e.g. tests or lint), the output will explicitly highlight the validation failures:

```text
--- Authoritative Truth (Summary): step-12345 ---
State:   failed_validation
Summary: One or more validations failed.

--- Validations ---
  [PASS] build-check      passed
  [FAIL] unit-tests       failed
         Error: Command failed: exit status 1
         Output: ...
```

## Immutable History (Attempts)

Every time you re-dispatch or retry a step, Codencer creates a **new, namespaced attempt directory**. Prior evidence is never overwritten.

To see all artifacts across all attempts:
```bash
./bin/orchestratorctl step artifacts <step-uuid>
```

The output groups files by attempt ID, showing you exactly how the repository evolved:

```text
Attempt step-123-a1-1712345000:
  Directory: .codencer/artifacts/run-1/step-123/step-123-a1-1712345000
  - [log] stdout.log
  - [result] result.json

Attempt step-123-a1-1712345123:
  Directory: .codencer/artifacts/run-1/step-123/step-123-a1-1712345123
  - [log] stdout.log
  - [result] result.json
```

## Validation Deep-Dive

To focus specifically on the history of tests and validations:
```bash
./bin/orchestratorctl step validations <step-uuid>
```

This ensures you can trace *why* an agent failed to meet its goal across different tries.
