# ADR 003: Codencer Native Workspace Provisioning

## Status: Proposed
## Decided by: principal engineer

## Decision
Codencer will implement a **native, lightweight workspace provisioning layer** to prepare attempt worktrees before execution. This layer will handle environment setup (copying `.env` files), dependency symlinking (e.g., `node_modules`), and optional post-creation hooks.

### Key Objectives
1. **Bridge, Not Brain**: Provisioning is a mechanical environment preparation step. It contains no planner logic and does not change the task goal.
2. **Local-First Consistency**: Ensure every execution attempt starts with the minimum local state required for success, even in isolated worktrees.
3. **Grove Inspiration**: Optionally read Grove-compatible configuration (`grove.yaml`) to reduce configuration duplication for users already using Grove.

## Minimal Feature Set (Phase 1-2)
- **File Copying**: Copy specifically allowlisted local files (e.g., `.env`, `config/secrets.json`) from the base repo to the attempt worktree.
- **Directory Symlinking**: Create symbolic links from the host's base repo to the attempt worktree for large, immutable, or shared-mutable directories (e.g., `node_modules`, `vendor`).
- **Post-Create Hooks**: Execute a single shell command (e.g., `npm install`) immediately after provisioning and before the executor starts.

## Config Model: `.codencer/workspace.json`
The primary source of truth for provisioning is a repo-local JSON file.

```json
{
  "provisioning": {
    "copy": [
      ".env",
      ".env.local"
    ],
    "symlinks": [
      "node_modules",
      "vendor"
    ],
    "hooks": {
      "post_create": "npm install --offline"
    }
  }
}
```

## Grove Compatibility
Codencer will **not** depend on the Grove CLI. Instead:
- If a Grove configuration (`grove.yaml`) exists, Codencer will attempt to read its `workspace.setup` and `workspace.hooks.post_create` sections.
- These values will be merged with `.codencer/workspace.json` (Codencer-native config takes precedence).
- This is a "read-only" compatibility layer; Codencer does not write Grove state files.

## Safety & Evidence
- **Allowlists**: Only paths explicitly listed in the config will be copied or symlinked.
- **Rollback**: Provisioning happens inside the attempt worktree. Cleanup of the worktree (existing behavior) implicitly rolls back provisioning.
- **Audit**: Provisioning actions (success/failure) will be logged to the `Attempt.Result.RawOutput` to distinguish setup failures from agent failures.

---

## Implementation Roadmap

### Phase 1: Skeleton & Spec (Next)
- Define `ProvisioningSpec` domain model.
- Define `Provisioner` interface in `internal/workspace`.
- Add "Setup Environment" stage to `RunService.executeAttempt` log.

### Phase 2: File & Symlink Implementation
- Implement `CopyFiles` and `CreateSymlinks` logic.
- Integrate into `executeAttempt` lifecycle.

### Phase 3: Hooks & Metadata
- Implement `PostCreate` hook execution.
- Capture provisioning success/failure in `Attempt.Result`.

### Phase 4: Grove Compatibility Detection
- Best-effort mapping from `grove.yaml` to `ProvisioningSpec`.
