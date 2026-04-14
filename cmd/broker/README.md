# Agent Broker

A lightweight same-side discovery and execution service for bridging Codencer (WSL) and Windows IDE-side agent contexts. The agent-broker acts as a headless, local-only proxy that handles LS discovery, workspace binding, and cascade execution.

## Configuration

The broker is configured via environment variables:

- `BROKER_HOST`: Host to bind to (default: `127.0.0.1`)
- `BROKER_PORT`: Port to listen on (default: `8088`)

## Build & Run

### 1. Build (Windows Host)
It is recommended to run the agent-broker natively on the Windows host where the IDE is running.
```powershell
# In PowerShell (Windows)
make build-broker
.\agent-broker.exe
```

### 2. Configure Codencer (WSL Guest)
Point Codencer to the broker's endpoint:
```bash
export CODENCER_ANTIGRAVITY_BROKER_URL=http://localhost:8088
```

Practical placement:
- keep the repo, daemon, connector, worktrees, and artifacts in WSL/Linux
- keep the broker on the Windows/IDE side
- keep the relay separate; it is not the broker

## API Reference

### Health & Discovery
- `GET /health`: Basic health check.
- `GET /version`: Version info.
- `GET /instances`: Lists all discovered agent-broker instances on the host.

### Binding Management
Binding is repo-specific. Each repository on the guest machine can be bound to a separate IDE-side instance.

- `GET /binding?repo_root=<path>`: Returns the active service instance for the repo.
- `POST /binding`: Bind repo to an instance (JSON: `{"pid": <PID>, "repo_root": "<path>"}`).
- `DELETE /binding?repo_root=<path>`: Clear the binding for the repo.

### Task Execution
- **POST /tasks**: Starts a cascade with isolated workspace support.
  - JSON: `{"prompt": "goal", "repo_root": "<stable_path>", "workspace_root": "<worktree_path>"}`
  - If `workspace_root` is provided, it is used for the LS execution context.
  - If omitted, it falls back to the instance default root discovered during bind.
- **GET /tasks/:id**: Polls status.
- **GET /tasks/:id/result**: Retrieves raw trajectory.

## Persistence
Binding state is persisted to `~/.gemini/antigravity/broker_binding.json` on the host machine.
Task sessions are currently kept in-memory; restarting the agent-broker will orphan active tasks.
