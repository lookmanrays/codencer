# Review Prompts and Acceptance Checklists

## Architecture audit prompt

```text
Audit the current implementation against:
- docs/02_architecture.md
- docs/04_repo_structure.md
- docs/09_ai_rules.md

Find:
- boundary violations
- misplaced logic
- provider leakage
- hidden coupling
- unnecessary complexity

Then fix only the highest-value issues for the current phase.
```

## Reliability audit prompt

```text
Audit the current implementation for reliability relative to:
- docs/07_security_ops.md

Focus:
- crash safety
- timeout handling
- cancellation
- retries
- duplicate execution
- stale lock handling
- invalid state transitions
- artifact durability

Then fix the highest-value issues and add tests.
```

## MCP audit prompt

```text
Audit the MCP server.

Check:
- only orchestrator primitives are exposed
- no raw shell escapes
- input validation is strict
- responses are compact and stable
- daemon remains source of truth
```

## Extension audit prompt

```text
Audit the VS Code extension.

Check:
- extension is thin
- daemon is source of truth
- no hidden business logic in extension
- gate UX is clear
- artifact access works
```

## Global acceptance checklist

### Design
- [ ] implementation matches current phase only
- [ ] architecture boundaries remain intact
- [ ] no provider-specific leakage into core contracts

### Correctness
- [ ] state transitions explicit
- [ ] errors surfaced, not swallowed
- [ ] artifacts persisted deterministically
- [ ] policy outcomes explainable

### Testing
- [ ] unit tests added or updated
- [ ] integration tests added where relevant
- [ ] relevant checks run and reported

### Docs
- [ ] docs updated if behavior changed
- [ ] limitations stated
- [ ] next risks identified

### Safety
- [ ] destructive actions gated
- [ ] repo safety considered
- [ ] cancellation/timeouts handled where relevant

## Phase-specific acceptance notes

### Core daemon
- [ ] daemon starts cleanly
- [ ] config validation works
- [ ] health/version endpoint works

### Ledger/state machine
- [ ] invalid transitions rejected
- [ ] audit trail persisted

### Codex adapter
- [ ] adapter implements common contract
- [ ] local invocation works
- [ ] timeout/cancellation story exists
- [ ] normalized result returned

### Policy engine
- [ ] gate creation works
- [ ] approve/reject works
- [ ] major gate paths covered by tests

### MCP
- [ ] safe tools only
- [ ] responses normalized
- [ ] error taxonomy stable
