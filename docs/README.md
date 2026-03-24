# Local Agent Bridge — Comprehensive Docset

This repository docset specifies a **local-first orchestration bridge for coding agents**.

Primary workflow:

1. Planning/design happens in an external AI chat.
2. The planner issues a structured step.
3. A local orchestrator executes the step through a local coding agent.
4. The orchestrator returns a structured result.
5. The planner decides whether to continue, retry, or stop for approval.

## Scope

Initial scope:
- local only
- CLI first
- Codex first
- no cloud in MVP
- no generic GUI automation in MVP
- IDE companion support later
- IDE chat execution adapters later

## Files

- `01_product_scope.md`
- `02_architecture.md`
- `03_roadmap.md`
- `04_repo_structure.md`
- `05_dsl_and_mcp.md`
- `06_adapters_and_ide.md`
- `07_security_ops.md`
- `08_market_research.md`
- `09_ai_rules.md`
- `10_implementation_prompts.md`
- `11_review_checklists.md`
- `references.md`

## Recommended use

1. Put this docset into a new repo.
2. Add a strong repo-root `AGENTS.md`.
3. In Antigravity IDE, ask Gemini 3.1 Thinking to read this docset first.
4. Implement **one phase at a time**.
5. After each phase, run the review prompts and acceptance checklist.

## Product definition

This is **not** another coding agent.

This is a **local control plane for coding agents** with:
- run / phase / step semantics
- durable ledger
- deterministic artifacts
- policy gates
- adapter abstraction
- future IDE companion surfaces
