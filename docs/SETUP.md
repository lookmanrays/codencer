# Workspace Provisioning & Isolation

Codencer uses Git Worktrees to isolate task attempts. To ensure these worktrees are ready for execution (e.g., they have the necessary `.env` files or `node_modules`), Codencer provides a native provisioning layer.

## Native Configuration: `.codencer/workspace.json`

Create a `.codencer/workspace.json` file in your repository root to define the provisioning plan.

```json
{
  "provisioning": {
    "copy": [
      ".env",
      ".env.local",
      "config/secrets.json"
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

### Configuration Fields

- **`copy`**: A list of relative paths to files that should be replicated from the base repository to the attempt worktree. This is ideal for small configuration or secret files that are not committed to git.
- **`symlinks`**: A list of relative paths to directories or files that should be symlinked from the base repository. This is critical for large dependency folders like `node_modules` to avoid the overhead of copying millions of files.
- **`hooks.post_create`**: A single shell command to run immediately after file preparation and before the agent starts. Use this for lightweight setup like `go mod download`.

## Grove Compatibility (Optional Subset)

Codencer includes an **optional compatibility layer** for [Grove](https://github.com/verbaux/grove) configuration. If you already use Grove for local development, Codencer will automatically detect and import your environment preparation settings if a native `.codencer/workspace.json` is not present. 

**Note**: Codencer does NOT depend on the Grove CLI and only reads the configuration files directly.

### Supported Grove Subset
Codencer only imports the environment preparation subset of Grove configuration. It ignores Grove's native lifecycle tracking, state files, and aliases.

The following fields are mapped:
- **Environment Replication**: `setup.copy` or `setup.env_files` -> `copy`
- **Dependency Optimization**: `setup.symlinks` or `symlink` -> `symlinks`
- **Preparation Hooks**: `hooks.post_create` or `afterCreate` -> `post_create`

### Precedence Rules
Codencer merges configuration from multiple sources using the following absolute priority:
1. **`.codencer/workspace.json`** (Codencer-native)
2. **`grove.yaml`** (Spec-Grove)
3. **`.groverc.json`** (Legacy-Grove)

If a field (e.g., `symlinks`) is defined in the native config, any definitions in Grove files for that specific field are ignored.

## Audit & Visibility

Every provisioning action is recorded in the attempt evidence. If provisioning fails, the attempt is marked as **`failed_bridge`**, and the reason is populated in the `StatusReason`.

You can inspect the setup logs using the CLI:

```bash
# View the high-level result (including provisioning summary)
./bin/orchestratorctl step result <UUID>
```

In the resulting JSON, look for the `provisioning` section:
```json
"provisioning": {
  "success": true,
  "summary": "",
  "log": [
    "Copy: .env -> .env",
    "Symlink: node_modules -> node_modules",
    "Hook (PostCreate): npm install --offline",
    "added 1 package in 2s..."
  ],
  "duration_ms": 2450
}
```
