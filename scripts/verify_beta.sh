#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERIFY_DOCKER_STACK=0
VERIFY_BETA_DAEMON_PORT="${VERIFY_BETA_DAEMON_PORT:-18085}"
VERIFY_BETA_RELAY_PORT="${VERIFY_BETA_RELAY_PORT:-18090}"
DAEMON_URL="http://127.0.0.1:${VERIFY_BETA_DAEMON_PORT}"
RELAY_URL="http://127.0.0.1:${VERIFY_BETA_RELAY_PORT}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/codencer-verify-beta.XXXXXX")"
RELAY_CONFIG="$TMP_DIR/relay.json"
PLANNER_TOKEN_JSON="$TMP_DIR/planner-token.json"
RELAY_LOG="$TMP_DIR/relay.log"
DAEMON_LOG="$TMP_DIR/daemon.log"
RELAY_PID=""
DAEMON_PID=""

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

json_get() {
  local file="$1"
  local expr="$2"
  if have_cmd jq; then
    jq -r "$expr" "$file"
    return
  fi
  if have_cmd python3; then
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
if value is None:
    value = ""
print(value)
PY
    return
  fi
  echo "ERROR: verify_beta.sh requires jq or python3 for JSON parsing." >&2
  exit 1
}

cleanup() {
  if [[ -n "$RELAY_PID" ]] && kill -0 "$RELAY_PID" >/dev/null 2>&1; then
    kill "$RELAY_PID" >/dev/null 2>&1 || true
    wait "$RELAY_PID" 2>/dev/null || true
  fi
  if [[ -n "$DAEMON_PID" ]] && kill -0 "$DAEMON_PID" >/dev/null 2>&1; then
    kill "$DAEMON_PID" >/dev/null 2>&1 || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

for arg in "$@"; do
  case "$arg" in
    --docker)
      VERIFY_DOCKER_STACK=1
      ;;
    *)
      echo "usage: $0 [--docker]" >&2
      exit 1
      ;;
  esac
done

cd "$ROOT_DIR"

echo "==> Running main-module tests..."
go test -v ./...

echo "==> Running local smoke..."
make smoke

cat > "$RELAY_CONFIG" <<EOF
{
  "host": "127.0.0.1",
  "port": ${VERIFY_BETA_RELAY_PORT},
  "db_path": "${TMP_DIR}/relay.db",
  "planner_tokens": []
}
EOF

echo "==> Creating temporary relay planner token config..."
./bin/codencer-relayd planner-token create \
  --config "$RELAY_CONFIG" \
  --write-config \
  --name verify-beta \
  --scope '*' \
  --json > "$PLANNER_TOKEN_JSON"

PLANNER_TOKEN="$(json_get "$PLANNER_TOKEN_JSON" '.token')"
if [[ -z "$PLANNER_TOKEN" ]]; then
  echo "ERROR: failed to create temporary planner token" >&2
  cat "$PLANNER_TOKEN_JSON" >&2
  exit 1
fi

echo "==> Starting temporary simulation daemon on ${DAEMON_URL}..."
nohup env ALL_ADAPTERS_SIMULATION_MODE=1 PORT="${VERIFY_BETA_DAEMON_PORT}" \
  ./bin/orchestratord --repo-root "$ROOT_DIR" > "$DAEMON_LOG" 2>&1 &
DAEMON_PID="$!"

for _ in $(seq 1 30); do
  if curl -fsS "${DAEMON_URL}/api/v1/instance" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! curl -fsS "${DAEMON_URL}/api/v1/instance" >/dev/null 2>&1; then
  echo "ERROR: verify-beta daemon failed to start on ${DAEMON_URL}" >&2
  cat "$DAEMON_LOG" >&2 || true
  exit 1
fi

echo "==> Starting temporary relay on ${RELAY_URL}..."
nohup ./bin/codencer-relayd --config "$RELAY_CONFIG" > "$RELAY_LOG" 2>&1 &
RELAY_PID="$!"

for _ in $(seq 1 30); do
  if ./bin/codencer-relayd status --config "$RELAY_CONFIG" --json >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! ./bin/codencer-relayd status --config "$RELAY_CONFIG" --json >/dev/null 2>&1; then
  echo "ERROR: verify-beta relay failed to start on ${RELAY_URL}" >&2
  cat "$RELAY_LOG" >&2 || true
  exit 1
fi

echo "==> Running self-host relay/runtime smoke with MCP + SDK..."
PLANNER_TOKEN="$PLANNER_TOKEN" \
RELAY_CONFIG="$RELAY_CONFIG" \
RELAY_URL="$RELAY_URL" \
DAEMON_URL="$DAEMON_URL" \
SMOKE_SCENARIOS="status,audit,mcp,mcp-sdk" \
./scripts/self_host_smoke.sh

echo "==> Running cloud binary smoke..."
make cloud-smoke

echo "==> Validating docker compose config..."
make cloud-stack-config

if [[ "$VERIFY_DOCKER_STACK" == "1" ]]; then
  echo "==> Running docker-backed cloud stack smoke..."
  make cloud-stack-smoke
fi

echo "==> Supported beta-track verification complete."
