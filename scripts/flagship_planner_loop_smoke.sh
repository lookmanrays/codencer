#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${BIN_DIR:-$ROOT_DIR/bin}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/codencer-flagship-planner.XXXXXX")"
DAEMON_PORT="${FLAGSHIP_DAEMON_PORT:-18185}"
GATE_DAEMON_PORT="${FLAGSHIP_GATE_DAEMON_PORT:-18186}"
RELAY_PORT="${FLAGSHIP_RELAY_PORT:-18190}"
DAEMON_URL="http://127.0.0.1:${DAEMON_PORT}"
GATE_DAEMON_URL="http://127.0.0.1:${GATE_DAEMON_PORT}"
RELAY_URL="http://127.0.0.1:${RELAY_PORT}"
RELAY_CONFIG="$TMP_DIR/relay.json"
PLANNER_TOKEN_JSON="$TMP_DIR/planner-token.json"
RELAY_LOG="$TMP_DIR/relay.log"
DAEMON_LOG="$TMP_DIR/daemon.log"
GATE_DAEMON_LOG="$TMP_DIR/gate-daemon.log"
RELAY_PID=""
DAEMON_PID=""
GATE_DAEMON_PID=""
PRIMARY_WORKTREE=""
GATE_WORKTREE=""

if [[ "${FLAGSHIP_LIVE_CODEX:-0}" == "1" ]]; then
  RELAY_PROXY_TIMEOUT_SECONDS="${FLAGSHIP_PROXY_TIMEOUT_SECONDS:-1800}"
  FLAGSHIP_WAIT_TIMEOUT_MS="${FLAGSHIP_WAIT_TIMEOUT_MS:-1800000}"
  FLAGSHIP_MCP_SDK_WAIT_TIMEOUT_MS="${FLAGSHIP_MCP_SDK_WAIT_TIMEOUT_MS:-1800000}"
else
  RELAY_PROXY_TIMEOUT_SECONDS="${FLAGSHIP_PROXY_TIMEOUT_SECONDS:-300}"
  FLAGSHIP_WAIT_TIMEOUT_MS="${FLAGSHIP_WAIT_TIMEOUT_MS:-300000}"
  FLAGSHIP_MCP_SDK_WAIT_TIMEOUT_MS="${FLAGSHIP_MCP_SDK_WAIT_TIMEOUT_MS:-10000}"
fi

json_get() {
  local file="$1"
  local expr="$2"
  if command -v jq >/dev/null 2>&1; then
    jq -r "$expr" "$file"
    return
  fi
  python3 - "$file" "$expr" <<'PY'
import json
import sys

path = sys.argv[1]
expr = sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    payload = json.load(handle)

value = payload
for part in expr.strip(".").split("."):
    if not part:
        continue
    if isinstance(value, dict):
        value = value.get(part, "")
    else:
        value = ""
        break
if isinstance(value, bool):
    value = "true" if value else "false"
elif value is None:
    value = ""
print(value)
PY
}

cleanup() {
  for pid in "$GATE_DAEMON_PID" "$DAEMON_PID" "$RELAY_PID"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  for worktree in "$GATE_WORKTREE" "$PRIMARY_WORKTREE"; do
    if [[ -n "$worktree" && -d "$worktree" ]]; then
      git -C "$ROOT_DIR" worktree remove -f "$worktree" >/dev/null 2>&1 || rm -rf "$worktree"
    fi
  done
  if [[ "${KEEP_FLAGSHIP_SMOKE_STATE:-0}" != "1" ]]; then
    rm -rf "$TMP_DIR"
  else
    echo "Retained flagship smoke state at $TMP_DIR"
  fi
}
trap cleanup EXIT

create_worktree() {
  local name="$1"
  local path="$TMP_DIR/$name"
  git -C "$ROOT_DIR" worktree add --detach "$path" HEAD >/dev/null
  printf '%s\n' "$path"
}

require_binary() {
  local path="$1"
  if [[ ! -x "$path" ]]; then
    echo "ERROR: expected executable $path. Run 'make build build-mcp-sdk-smoke' first." >&2
    exit 1
  fi
}

wait_for_url() {
  local url="$1"
  local log="$2"
  for _ in $(seq 1 30); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "ERROR: timed out waiting for $url" >&2
  cat "$log" >&2 || true
  exit 1
}

start_daemon() {
  local port="$1"
  local log="$2"
  local force_gate="$3"
  local repo_root="$4"
  local -a env_args=("PORT=$port")
  if [[ "${FLAGSHIP_LIVE_CODEX:-0}" == "1" ]]; then
    env_args+=("CODEX_BINARY=${CODEX_BINARY:-codex}" "ALL_ADAPTERS_SIMULATION_MODE=0" "CODEX_SIMULATION_MODE=0")
  else
    env_args+=("ALL_ADAPTERS_SIMULATION_MODE=1")
  fi
  if [[ "$force_gate" == "1" ]]; then
    env_args+=("FORCE_GATE_FOR_TESTING=1")
  fi
  nohup env "${env_args[@]}" "$BIN_DIR/orchestratord" --repo-root "$repo_root" > "$log" 2>&1 &
  echo "$!"
}

require_binary "$BIN_DIR/orchestratord"
require_binary "$BIN_DIR/codencer-relayd"
require_binary "$BIN_DIR/codencer-connectord"

cat > "$RELAY_CONFIG" <<EOF
{
  "host": "127.0.0.1",
  "port": ${RELAY_PORT},
  "db_path": "${TMP_DIR}/relay.db",
  "planner_tokens": [],
  "public_base_url": "${RELAY_URL}",
  "oauth_authorization_servers": ["https://auth.example.invalid"],
  "oauth_scopes_supported": [
    "instances:read",
    "runs:read",
    "runs:write",
    "steps:read",
    "steps:write",
    "artifacts:read",
    "gates:read",
    "gates:write"
  ],
  "oauth_resource_documentation": "https://example.invalid/codencer/relay-mcp",
  "proxy_timeout_seconds": ${RELAY_PROXY_TIMEOUT_SECONDS}
}
EOF

"$BIN_DIR/codencer-relayd" planner-token create \
  --config "$RELAY_CONFIG" \
  --write-config \
  --name flagship-planner \
  --scope '*' \
  --json > "$PLANNER_TOKEN_JSON"

PLANNER_TOKEN="$(json_get "$PLANNER_TOKEN_JSON" '.token')"
if [[ -z "$PLANNER_TOKEN" ]]; then
  echo "ERROR: failed to create planner token." >&2
  cat "$PLANNER_TOKEN_JSON" >&2
  exit 1
fi

"$BIN_DIR/codencer-relayd" --config "$RELAY_CONFIG" > "$RELAY_LOG" 2>&1 &
RELAY_PID="$!"
for _ in $(seq 1 30); do
  if "$BIN_DIR/codencer-relayd" status --config "$RELAY_CONFIG" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! "$BIN_DIR/codencer-relayd" status --config "$RELAY_CONFIG" >/dev/null 2>&1; then
  echo "ERROR: timed out waiting for relay at $RELAY_URL" >&2
  cat "$RELAY_LOG" >&2 || true
  exit 1
fi

PRIMARY_WORKTREE="$(create_worktree primary)"
DAEMON_PID="$(start_daemon "$DAEMON_PORT" "$DAEMON_LOG" 0 "$PRIMARY_WORKTREE")"
wait_for_url "$DAEMON_URL/api/v1/instance" "$DAEMON_LOG"

echo "==> Flagship relay planner loop: single-step, phase-loop, multi-instance, reconnect, MCP, SDK"
if [[ "${FLAGSHIP_LIVE_CODEX:-0}" == "1" ]]; then
  CODEX_BINARY="${CODEX_BINARY:-codex}" \
  ALL_ADAPTERS_SIMULATION_MODE=0 \
  CODEX_SIMULATION_MODE=0 \
  REQUIRE_LIVE_CODEX=1 \
  PLANNER_TOKEN="$PLANNER_TOKEN" \
  RELAY_CONFIG="$RELAY_CONFIG" \
  RELAY_URL="$RELAY_URL" \
  DAEMON_URL="$DAEMON_URL" \
  CONNECTOR_ADAPTER=codex \
  WAIT_TIMEOUT_MS="$FLAGSHIP_WAIT_TIMEOUT_MS" \
  MCP_SDK_STEP_COUNT=2 \
  MCP_SDK_WAIT_TIMEOUT_MS="$FLAGSHIP_MCP_SDK_WAIT_TIMEOUT_MS" \
  SMOKE_SCENARIOS=status,audit,share-control,multi-instance,reconnect,phase-loop,mcp-auth-metadata,mcp,mcp-sdk \
  "$ROOT_DIR/scripts/self_host_smoke.sh"
else
  PLANNER_TOKEN="$PLANNER_TOKEN" \
  RELAY_CONFIG="$RELAY_CONFIG" \
  RELAY_URL="$RELAY_URL" \
  DAEMON_URL="$DAEMON_URL" \
  CONNECTOR_ADAPTER=codex \
  WAIT_TIMEOUT_MS="$FLAGSHIP_WAIT_TIMEOUT_MS" \
  MCP_SDK_STEP_COUNT=2 \
  MCP_SDK_WAIT_TIMEOUT_MS="$FLAGSHIP_MCP_SDK_WAIT_TIMEOUT_MS" \
  SMOKE_SCENARIOS=status,audit,share-control,multi-instance,reconnect,phase-loop,mcp-auth-metadata,mcp,mcp-sdk \
  "$ROOT_DIR/scripts/self_host_smoke.sh"
fi

GATE_WORKTREE="$(create_worktree gate)"
GATE_DAEMON_PID="$(start_daemon "$GATE_DAEMON_PORT" "$GATE_DAEMON_LOG" 1 "$GATE_WORKTREE")"
wait_for_url "$GATE_DAEMON_URL/api/v1/instance" "$GATE_DAEMON_LOG"

echo "==> Flagship relay planner loop: strict gate approval"
if [[ "${FLAGSHIP_LIVE_CODEX:-0}" == "1" ]]; then
  CODEX_BINARY="${CODEX_BINARY:-codex}" \
  ALL_ADAPTERS_SIMULATION_MODE=0 \
  CODEX_SIMULATION_MODE=0 \
  REQUIRE_LIVE_CODEX=1 \
  PLANNER_TOKEN="$PLANNER_TOKEN" \
  RELAY_CONFIG="$RELAY_CONFIG" \
  RELAY_URL="$RELAY_URL" \
  DAEMON_URL="$GATE_DAEMON_URL" \
  CONNECTOR_ADAPTER=codex \
  WAIT_TIMEOUT_MS="$FLAGSHIP_WAIT_TIMEOUT_MS" \
  GATE_ACTION=approve \
  RUN_ID="flagship-gate-$(date +%s)" \
  SMOKE_SCENARIOS=gate-strict \
  "$ROOT_DIR/scripts/self_host_smoke.sh"
else
  PLANNER_TOKEN="$PLANNER_TOKEN" \
  RELAY_CONFIG="$RELAY_CONFIG" \
  RELAY_URL="$RELAY_URL" \
  DAEMON_URL="$GATE_DAEMON_URL" \
  CONNECTOR_ADAPTER=codex \
  WAIT_TIMEOUT_MS="$FLAGSHIP_WAIT_TIMEOUT_MS" \
  GATE_ACTION=approve \
  RUN_ID="flagship-gate-$(date +%s)" \
  SMOKE_SCENARIOS=gate-strict \
  "$ROOT_DIR/scripts/self_host_smoke.sh"
fi

echo "Flagship planner loop smoke completed successfully."
