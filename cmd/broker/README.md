# Antigravity Discovery Broker

A lightweight same-side discovery and execution service for bridging Codencer (WSL) and Antigravity (Windows). The broker acts as a headless, local-only proxy that handles LS discovery, workspace binding, and cascade execution.

## Configuration

The broker is configured via environment variables:

- `BROKER_HOST`: Host to bind to (default: `127.0.0.1`)
- `BROKER_PORT`: Port to listen on (default: `8088`)

## Build & Run

### 1. Build (Windows Host)
It is recommended to run the broker natively on the Windows host where the IDE is running.
```powershell
# In PowerShell (Windows)
go build -o agent-broker.exe main.go
.\agent-broker.exe
```

### 2. Configure Codencer (WSL Guest)
Point Codencer to the broker's endpoint:
```bash
export CODENCER_ANTIGRAVITY_BROKER_URL=http://localhost:8088
```

## API Reference

### Health & Discovery
- `GET /health`: Basic health check.
- `GET /version`: Version info.
- `GET /instances`: Lists all discovered Antigravity instances on the host.

### Binding Management
- `GET /binding`: Returns the current repository binding.
- `POST /binding`: Bind to a specific instance (JSON: `{"pid": <PID>}`).
- `DELETE /binding`: Clear the current binding.

### Task Execution
- `POST /tasks`: Starts an Antigravity cascade (JSON: `{"prompt": "goal"}`).
- `GET /tasks/:id`: Polls task status and trajectory.
- `GET /tasks/:id/result`: Retrieves the final task result and terminal state.

## Persistence
Binding state is persisted to `~/.gemini/antigravity/broker_binding.json` on the host machine.
Task sessions are currently kept in-memory; restarting the broker will orphan active tasks.
