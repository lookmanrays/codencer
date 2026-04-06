# Workspace Provisioning: Common Examples

This guide provides configuration templates for common project types to ensure your isolated worktrees are ready for agents.

## Node.js / TypeScript

Efficiently share your `node_modules` avoiding costly file copies.

```json
{
  "provisioning": {
    "copy": [".env", ".env.local"],
    "symlinks": ["node_modules"],
    "hooks": {
      "post_create": "npm install --offline"
    }
  }
}
```

## Go / Modules

Ensure the vendor directory is present or dependencies are downloaded.

```json
{
  "provisioning": {
    "copy": [".env", "config/secrets.json"],
    "symlinks": ["vendor"],
    "hooks": {
      "post_create": "go mod download"
    }
  }
}
```

## Python / Pipenv

Link your virtual environment and copy your `.env` file.

```json
{
  "provisioning": {
    "copy": [".env"],
    "symlinks": [".venv"],
    "hooks": {
      "post_create": "pipenv install --deploy --ignore-pipfile"
    }
  }
}
```

## Grove Compatibility (Zero-Config Merging)

If your project already has a `.groverc.json`, Codencer automatically leverages it:

### `.groverc.json` (Existing)
```json
{
  "symlink": ["node_modules", "dist"],
  "afterCreate": "make setup"
}
```

### Resulting Codencer Spec
- **Copy**: `[]` (None defined in Grove)
- **Symlinks**: `["node_modules", "dist"]`
- **Hooks**: `{ "post_create": "make setup" }`

### Overriding Grove for Codencer
If you need to change only the hook for Codencer, add a native config:

```json
{
  "provisioning": {
    "hooks": {
      "post_create": "make codencer-special-setup"
    }
  }
}
```
**Precedence**: Codencer will now use your native hook but still pull the symlink list from Grove.

---

## Full Provisioning Walkthrough

This example demonstrates how to prepare a project for an agent-driven bug fix where local secrets and heavy dependencies are required.

### 1. Setup the Repository Config
Create `.codencer/workspace.json` in your repository root:
```json
{
  "provisioning": {
    "copy": [".env"],
    "symlinks": ["node_modules"],
    "hooks": {
      "post_create": "npm install --offline"
    }
  }
}
```

### 2. Submit a Task
Submit a task that requires these dependencies (e.g., running a test):
```bash
./bin/orchestratorctl submit my-run-1 examples/tasks/fix_auth_bug.yaml --wait
```

### 3. What Happens Under the Hood
1. **Worktree Creation**: Codencer creates a new git worktree for the attempt.
2. **Provisioning**: 
    - Replication: `.env` is copied from the base repo.
    - Optimization: `node_modules/` is symlinked (no file copy overhead).
    - Preparation: `npm install --offline` runs to verify dependencies.
3. **Execution**: The agent starts in the provisioned environment.
4. **Validation**: The bug fix is validated against the local test suite.

### 4. Inspect the Evidence
Verify the setup using the step UUID:
```bash
./bin/orchestratorctl step result <UUID>
```
The output will contain the detailed `provisioning` log, proving that the environment was correctly prepared.
