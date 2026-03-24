# Implementation Prompts

Use these prompts in Antigravity IDE with Gemini 3.1 Thinking / Flash.

## Master bootstrap prompt

```text
You are implementing a production-oriented local orchestration bridge for coding agents.

Read these files first and treat them as the system specification:
- docs/01_product_scope.md
- docs/02_architecture.md
- docs/03_roadmap.md
- docs/04_repo_structure.md
- docs/05_dsl_and_mcp.md
- docs/06_adapters_and_ide.md
- docs/07_security_ops.md
- docs/08_market_research.md
- docs/09_ai_rules.md
- docs/11_review_checklists.md

Task:
Bootstrap the repository according to docs/04_repo_structure.md.

Requirements:
- create the folder structure
- initialize the Go module
- add minimal but clean bootstrap code for cmd entrypoints
- add config loading foundation
- add logging foundation
- add SQLite migration scaffolding
- add test scaffolding
- add Makefile targets
- add repo-root AGENTS.md aligned with docs/09_ai_rules.md
- do not implement future provider logic yet
- do not add cloud functionality
- do not add web UI

Output:
1. implementation summary
2. file tree summary
3. key design decisions
4. commands run
5. known TODOs for next phase
```

## Generic phase execution prompt

```text
Implement the next requested phase only.

Process:
1. restate the phase objective
2. identify exact files/modules to create or modify
3. implement narrowly and cleanly
4. add or update tests
5. update docs if needed
6. run relevant checks
7. return:
   - what changed
   - why
   - what remains intentionally out of scope
   - risks or caveats

Strict rules:
- do not widen scope
- do not skip tests
- do not skip error handling
- do not silently break previous phases
- do not create fake success states
```

## Architecture guard prompt

```text
Before coding, validate your plan against the architecture docs.

Reject your own approach if it causes any of these:
- provider-specific logic leaking into core orchestration contracts
- the IDE layer becoming the orchestrator
- persistence being bypassed
- untyped ad hoc task/result payloads
- policy logic scattered outside policy engine
- hidden global mutable state

Then proceed with the narrowest correct implementation.
```

## Phase prompts

### Phase 1 prompt
```text
Implement Phase 1 from docs/03_roadmap.md.

Focus:
- daemon bootstrap
- config loading
- structured logging
- health endpoint
- version endpoint
- SQLite init scaffold
- artifact root initialization

Do not implement adapters yet.
Do not implement MCP yet.
Do not implement IDE functionality yet.

Add tests where practical for config/bootstrap behavior.
```

### Phase 2 prompt
```text
Implement Phase 2 from docs/03_roadmap.md.

Focus:
- domain entities
- SQLite schema for runs/phases/steps/attempts/artifacts/gates
- repository layer
- state machine
- transition validation
- audit trail

Requirements:
- invalid transitions must be rejected
- transitions must be explicit and test-covered
```

### Phase 3 prompt
```text
Implement Phase 3 from docs/03_roadmap.md.

Focus:
- orchestratorctl
- run start/status
- step start/result
- gate approve/reject
- abort commands

Requirements:
- no direct DB access from CLI handlers
- use service layer
- clear exit codes
```

### Phase 4 prompt
```text
Implement Phase 4 from docs/03_roadmap.md.

Focus:
- adapter interface
- Codex adapter package
- task rendering
- local invocation
- process supervision
- timeout handling
- normalized result skeleton

Requirements:
- keep provider-specific logic inside internal/adapters/codex/
- do not add Claude or Qwen support yet
```

### Phase 5 prompt
```text
Implement Phase 5 from docs/03_roadmap.md.

Focus:
- validation runner
- diff collection
- changed file collection
- artifact persistence
- validation result persistence
```

### Phase 6 prompt
```text
Implement Phase 6 from docs/03_roadmap.md.

Focus:
- policy evaluator
- gate creation
- approve/reject flow
- retry semantics

Initial policy conditions:
- validation failures
- dependency changes
- migration detection
- forbidden path touches
- file count threshold
- unresolved questions
```

### Phase 7 prompt
```text
Implement Phase 7 from docs/03_roadmap.md.

Focus:
- repo cleanliness checks
- worktree allocation strategy
- locks
- cleanup path
- run-specific workspace handling
```

### Phase 8 prompt
```text
Implement Phase 8 from docs/03_roadmap.md.

Focus:
- daemon restart recovery
- stale process detection
- resumable runs
- cancellation
- retry backoff
- idempotency safeguards
```

### Phase 9 prompt
```text
Implement Phase 9 from docs/03_roadmap.md.

Focus:
- local MCP server
- tool mapping
- request/response contracts
- error normalization

Requirements:
- expose orchestrator tools only
- do not expose raw shell access
```

### Phase 10 prompt
```text
Implement Phase 10 from docs/03_roadmap.md.

Focus:
- JSON/YAML schemas
- parser/validator
- semantic validation
- examples
```

### Phase 11 prompt
```text
Implement Phase 11 from docs/03_roadmap.md.

Focus:
- Claude adapter
- capability metadata
- normalization
- conformance tests
```

### Phase 12 prompt
```text
Implement Phase 12 from docs/03_roadmap.md.

Focus:
- Qwen adapter
- capability metadata
- normalization
- conformance tests
```

### Phase 14 prompt
```text
Implement Phase 14 from docs/03_roadmap.md.

Focus:
- VS Code extension skeleton
- daemon communication
- run/step/gate views
- approval/retry controls
- artifact links

Requirements:
- extension is a thin control surface
- no orchestration state ownership in extension
```

### Phase 15 prompt
```text
Implement Phase 15 from docs/03_roadmap.md.

Focus:
- one supported IDE/agent chat adapter only
- submission pathway
- completion detection
- transcript/result capture
- fallback to CLI adapter

Requirements:
- do not claim universal support
- document reliability assumptions and failure modes
```
