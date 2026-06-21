# Project Config

Codencer stores commit-safe project defaults in:

```text
repo/.codencer/project.json
```

By default, `codencer project init` creates only this file under repo `.codencer/`. It does not create policy files, manifests, prompts, schemas, logs, runtime state, artifacts, proof bundles, connector identity, daemon URLs, relay URLs, tokens, private keys, enrollment material, machine IDs, or absolute local paths in the repository.

Gateway config and Relay profiles are local runtime state under
`$CODENCER_HOME/runtime/gateway/config.json`; they are never written into
`repo/.codencer/project.json`.

## Schema

Minimum shape:

```json
{
  "version": "codencer.io/v1alpha1",
  "kind": "ProjectConfig",
  "project": {
    "id": "codencer",
    "name": "Codencer",
    "description": ""
  },
  "execution": {
    "default_adapter": "codex",
    "default_profile": "codex-workspace"
  },
  "workspace": {
    "root": ".",
    "forbidden_paths": [".env", ".env.*", ".git", "node_modules", "dist", "build"]
  },
  "validations": []
}
```

`workspace.root` and forbidden paths must be relative repository paths. Project config validation rejects absolute paths, URLs, and token-like fields such as `token`, `secret`, `private_key`, `daemon_url`, `relay_url`, `connector_id`, and `machine_id`.

## Workspace provisioning and Grove compatibility

Workspace provisioning is opt-in and separate from `.codencer/project.json`.
Codencer reads native provisioning settings from `.codencer/workspace.json` when
that file exists, but `codencer project init` does not create it by default.

Codencer is Grove-compatible. Codencer can read a safe subset of `grove.yaml`
and `.groverc.json`. It uses those files only if native
`.codencer/workspace.json` is absent or incomplete. Native
`.codencer/workspace.json` has precedence. Codencer does not depend on the Grove
CLI. Codencer does not write Grove state files.

Current Grove-compatible reads are limited to provisioning-oriented fields:

- `grove.yaml`: `workspace.setup.copy`, `workspace.setup.symlinks`, and
  `workspace.hooks.post_create`.
- `.groverc.json`: `symlink` and `afterCreate`.

Codencer merges these as fallbacks only for missing native provisioning fields;
it does not treat Grove files as project identity, routing, credential, daemon,
Relay, or connector configuration.

## Local State

Machine-local state stays in `$CODENCER_HOME`:

- `$CODENCER_HOME/machine.json`: stable local `machine_id`, detected `hostname`, editable `host_label`, OS, arch, timestamps.
- `$CODENCER_HOME/projects.json`: local registry with absolute `repo_root`, daemon URL, sharing state, relay instance mapping, machine metadata, and project config path.
- `$CODENCER_HOME/runtime/gateway/config.json`: local Gateway config with Relay profile URLs and token env/file references.

Use:

```bash
codencer machine show --json
codencer machine set-label macbook --json
```

`host_label` is safe to edit and is used by Relay/MCP as a human-friendly project-location selector. It is not committed.

## Init, Adopt, Scan

When `.codencer/project.json` is missing:

```bash
codencer project init --repo . --id codencer --name "Codencer" --json
```

creates `.codencer/project.json`, validates it, generates or loads machine identity, and creates/updates only the local registry.

When `.codencer/project.json` already exists, `project init` validates and adopts it into `$CODENCER_HOME/projects.json`. It does not overwrite the repo config unless `--update-project-config` or `--force` is explicit.

After cloning a repo that already has project config:

```bash
codencer project adopt --repo . --json
```

requires `.codencer/project.json` and updates only local registry state.

Scan is read-only:

```bash
codencer project scan --repo . --json
```

It detects common project files and proposes language, package manager, commands, forbidden paths, and project id/name. It does not write files.

## Multi-Machine Routing

Connectors advertise shared projects with location metadata:

- `project_id`
- `project_name`
- `machine_id`
- `host_label`
- `hostname`
- `instance_id`
- `status`

Gateway and Relay/MCP project listings return projects with `locations[]`.
Locations include machine and connector identifiers plus safe repo labels/hashes,
never absolute repo paths.

Project execution tools accept optional selectors:

```json
{"project_id": "codencer", "machine_id": "mach_..."}
```

or:

```json
{"project_id": "codencer", "host_label": "macbook"}
```

Resolution order:

1. `machine_id` routes to that online location.
2. `host_label` routes to a matching online location.
3. A single online location routes automatically.
4. Multiple online locations without a selector return `ambiguous_project_location` with `planner_decision_required: true`.

Codencer never randomly chooses among multiple machines for the same `project_id`.

Gateway adds one more selector layer: if multiple Relay profiles expose the same
`project_id`, execution must pass `relay_profile_id` or Gateway returns
`ambiguous_relay_profile`.
