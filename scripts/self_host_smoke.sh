#!/usr/bin/env bash
set -euo pipefail

DAEMON_URL="${DAEMON_URL:-http://127.0.0.1:8085}"
RELAY_URL="${RELAY_URL:-http://127.0.0.1:8090}"
PLANNER_TOKEN="${PLANNER_TOKEN:-}"
CONNECTOR_LABEL="${CONNECTOR_LABEL:-self-host-smoke}"
CONNECTOR_ADAPTER="${CONNECTOR_ADAPTER:-codex}"
RUN_ID="${RUN_ID:-smoke-run-$(date +%s)}"
PROJECT_ID="${PROJECT_ID:-smoke-project}"
KEEP_SMOKE_STATE="${KEEP_SMOKE_STATE:-0}"

if [[ -z "$PLANNER_TOKEN" ]]; then
  echo "ERROR: PLANNER_TOKEN is required." >&2
  exit 1
fi

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
  echo "ERROR: self_host_smoke.sh requires jq or python3 for JSON parsing." >&2
  exit 1
}

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/codencer-selfhost.XXXXXX")"
CONNECTOR_CONFIG="${CONNECTOR_CONFIG:-$TMP_DIR/connector.json}"
CONNECTOR_PID=""

cleanup() {
  if [[ -n "$CONNECTOR_PID" ]] && kill -0 "$CONNECTOR_PID" >/dev/null 2>&1; then
    kill "$CONNECTOR_PID" >/dev/null 2>&1 || true
    wait "$CONNECTOR_PID" 2>/dev/null || true
  fi
  if [[ "$KEEP_SMOKE_STATE" != "1" ]]; then
    rm -rf "$TMP_DIR"
  else
    echo "Retained smoke state at $TMP_DIR"
  fi
}
trap cleanup EXIT

echo "--- Codencer Self-Host Smoke ---"
echo "Daemon: $DAEMON_URL"
echo "Relay:  $RELAY_URL"

INSTANCE_JSON="$TMP_DIR/instance.json"
curl -fsS "$DAEMON_URL/api/v1/instance" > "$INSTANCE_JSON"
INSTANCE_ID="$(json_get "$INSTANCE_JSON" '.id')"
if [[ -z "$INSTANCE_ID" ]]; then
  echo "ERROR: failed to read daemon instance id from $DAEMON_URL/api/v1/instance" >&2
  exit 1
fi
echo "Local instance: $INSTANCE_ID"

TOKEN_JSON="$TMP_DIR/enrollment-token.json"
curl -fsS -X POST "$RELAY_URL/api/v2/connectors/enrollment-tokens" \
  -H "Authorization: Bearer $PLANNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"label\":\"$CONNECTOR_LABEL\",\"expires_in_seconds\":600}" \
  > "$TOKEN_JSON"
ENROLLMENT_TOKEN="$(json_get "$TOKEN_JSON" '.secret')"
if [[ -z "$ENROLLMENT_TOKEN" ]]; then
  echo "ERROR: failed to create enrollment token" >&2
  cat "$TOKEN_JSON" >&2
  exit 1
fi

./bin/codencer-connectord enroll \
  --relay-url "$RELAY_URL" \
  --daemon-url "$DAEMON_URL" \
  --enrollment-token "$ENROLLMENT_TOKEN" \
  --config "$CONNECTOR_CONFIG" \
  --label "$CONNECTOR_LABEL" >/dev/null

./bin/codencer-connectord run --config "$CONNECTOR_CONFIG" >"$TMP_DIR/connector.log" 2>&1 &
CONNECTOR_PID="$!"

INSTANCES_JSON="$TMP_DIR/instances.json"
for _ in $(seq 1 20); do
  curl -fsS "$RELAY_URL/api/v2/instances" \
    -H "Authorization: Bearer $PLANNER_TOKEN" > "$INSTANCES_JSON"
  if grep -q "\"instance_id\":\"$INSTANCE_ID\"" "$INSTANCES_JSON"; then
    break
  fi
  sleep 1
done

if ! grep -q "\"instance_id\":\"$INSTANCE_ID\"" "$INSTANCES_JSON"; then
  echo "ERROR: connector did not advertise instance $INSTANCE_ID" >&2
  cat "$INSTANCES_JSON" >&2
  exit 1
fi

echo "Connector status:"
./bin/codencer-connectord status --config "$CONNECTOR_CONFIG" --json

RUN_JSON="$TMP_DIR/run.json"
curl -fsS -X POST "$RELAY_URL/api/v2/instances/$INSTANCE_ID/runs" \
  -H "Authorization: Bearer $PLANNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":\"$RUN_ID\",\"project_id\":\"$PROJECT_ID\"}" \
  > "$RUN_JSON"

STEP_JSON="$TMP_DIR/step.json"
curl -fsS -X POST "$RELAY_URL/api/v2/instances/$INSTANCE_ID/runs/$RUN_ID/steps" \
  -H "Authorization: Bearer $PLANNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"version\":\"v1\",\"goal\":\"Verify the self-host relay path\",\"adapter_profile\":\"$CONNECTOR_ADAPTER\"}" \
  > "$STEP_JSON"
STEP_ID="$(json_get "$STEP_JSON" '.id')"
if [[ -z "$STEP_ID" ]]; then
  echo "ERROR: failed to create step through relay" >&2
  cat "$STEP_JSON" >&2
  exit 1
fi

WAIT_JSON="$TMP_DIR/wait.json"
curl -fsS -X POST "$RELAY_URL/api/v2/steps/$STEP_ID/wait" \
  -H "Authorization: Bearer $PLANNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"timeout_ms":300000,"interval_ms":1000,"include_result":false}' \
  > "$WAIT_JSON"

RESULT_JSON="$TMP_DIR/result.json"
curl -fsS "$RELAY_URL/api/v2/steps/$STEP_ID/result" \
  -H "Authorization: Bearer $PLANNER_TOKEN" > "$RESULT_JSON"

ARTIFACTS_JSON="$TMP_DIR/artifacts.json"
curl -fsS "$RELAY_URL/api/v2/steps/$STEP_ID/artifacts" \
  -H "Authorization: Bearer $PLANNER_TOKEN" > "$ARTIFACTS_JSON"

echo "--- Self-Host Smoke Summary ---"
echo "Run:      $RUN_ID"
echo "Step:     $STEP_ID"
echo "State:    $(json_get "$WAIT_JSON" '.state')"
echo "Terminal: $(json_get "$WAIT_JSON" '.terminal')"
echo "Summary:  $(json_get "$RESULT_JSON" '.summary')"
echo "Artifacts recorded in: $ARTIFACTS_JSON"
