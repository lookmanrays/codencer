#!/usr/bin/env bash
set -euo pipefail

DAEMON_URL="${DAEMON_URL:-http://127.0.0.1:8085}"
RELAY_URL="${RELAY_URL:-http://127.0.0.1:8090}"
RELAY_CONFIG="${RELAY_CONFIG:-}"
PLANNER_TOKEN="${PLANNER_TOKEN:-}"
CONNECTOR_LABEL="${CONNECTOR_LABEL:-self-host-smoke}"
CONNECTOR_ADAPTER="${CONNECTOR_ADAPTER:-codex}"
RUN_ID="${RUN_ID:-smoke-run-$(date +%s)}"
PROJECT_ID="${PROJECT_ID:-smoke-project}"
KEEP_SMOKE_STATE="${KEEP_SMOKE_STATE:-0}"
SMOKE_SCENARIOS="${SMOKE_SCENARIOS:-status,audit}"
GATE_ACTION="${GATE_ACTION:-approve}"

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

json_first_field() {
  local file="$1"
  local field="$2"
  if have_cmd jq; then
    jq -r "if length > 0 then .[0].${field} // \"\" else \"\" end" "$file"
    return
  fi
  if have_cmd python3; then
    python3 - "$file" "$field" <<'PY'
import json
import sys

path = sys.argv[1]
field = sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    payload = json.load(handle)

value = ""
if isinstance(payload, list) and payload:
    first = payload[0]
    if isinstance(first, dict):
        value = first.get(field, "") or ""
print(value)
PY
    return
  fi
  echo "ERROR: self_host_smoke.sh requires jq or python3 for array JSON parsing." >&2
  exit 1
}

relay_config_token() {
  local path="$1"
  if [[ -z "$path" || ! -f "$path" ]]; then
    return
  fi
  if have_cmd jq; then
    jq -r '.planner_token // .planner_tokens[0].token // ""' "$path"
    return
  fi
  if have_cmd python3; then
    python3 - "$path" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)

token = payload.get("planner_token", "") or ""
if not token:
    tokens = payload.get("planner_tokens") or []
    if tokens and isinstance(tokens[0], dict):
        token = tokens[0].get("token", "") or ""
print(token)
PY
    return
  fi
}

scenario_enabled() {
  case ",$SMOKE_SCENARIOS," in
    *,all,*|*,"$1",*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

relay_target_args() {
  local args=()
  if [[ -n "$RELAY_CONFIG" ]]; then
    args+=(--config "$RELAY_CONFIG")
  fi
  args+=(--relay-url "$RELAY_URL")
  if [[ -n "$PLANNER_TOKEN" ]]; then
    args+=(--token "$PLANNER_TOKEN")
  fi
  printf '%s\n' "${args[@]}"
}

relay_cli() {
  local cmd=("$@")
  local target=()
  while IFS= read -r line; do
    target+=("$line")
  done < <(relay_target_args)
  ./bin/codencer-relayd "${cmd[@]}" "${target[@]}"
}

curl_json() {
  local method="$1"
  local url="$2"
  local outfile="$3"
  local data="${4:-}"
  local curl_args=(-fsS -X "$method" "$url" -H "Authorization: Bearer $PLANNER_TOKEN")
  if [[ -n "$data" ]]; then
    curl_args+=(-H 'Content-Type: application/json' -d "$data")
  fi
  curl "${curl_args[@]}" > "$outfile"
}

curl_best_effort() {
  local method="$1"
  local url="$2"
  local outfile="$3"
  local data="${4:-}"
  local curl_args=(-sS -X "$method" "$url" -H "Authorization: Bearer $PLANNER_TOKEN" -o "$outfile" -w "%{http_code}")
  if [[ -n "$data" ]]; then
    curl_args+=(-H 'Content-Type: application/json' -d "$data")
  fi
  curl "${curl_args[@]}"
}

if [[ -z "$PLANNER_TOKEN" && -n "$RELAY_CONFIG" ]]; then
  PLANNER_TOKEN="$(relay_config_token "$RELAY_CONFIG")"
fi
if [[ -z "$PLANNER_TOKEN" ]]; then
  echo "ERROR: PLANNER_TOKEN is required, or RELAY_CONFIG must contain planner_token(s)." >&2
  exit 1
fi

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
echo "Daemon:    $DAEMON_URL"
echo "Relay:     $RELAY_URL"
echo "Scenarios: $SMOKE_SCENARIOS"

INSTANCE_JSON="$TMP_DIR/instance.json"
curl -fsS "$DAEMON_URL/api/v1/instance" > "$INSTANCE_JSON"
INSTANCE_ID="$(json_get "$INSTANCE_JSON" '.id')"
if [[ -z "$INSTANCE_ID" ]]; then
  echo "ERROR: failed to read daemon instance id from $DAEMON_URL/api/v1/instance" >&2
  exit 1
fi
echo "Local instance: $INSTANCE_ID"

if scenario_enabled status; then
  relay_cli status --json > "$TMP_DIR/relay-status-before.json"
fi

TOKEN_JSON="$TMP_DIR/enrollment-token.json"
relay_cli enrollment-token create --label "$CONNECTOR_LABEL" --expires-in-seconds 600 --json > "$TOKEN_JSON"
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

./bin/codencer-connectord list --config "$CONNECTOR_CONFIG" > "$TMP_DIR/connector-list.txt"
./bin/codencer-connectord config --config "$CONNECTOR_CONFIG" > "$TMP_DIR/connector-config.txt"
./bin/codencer-connectord run --config "$CONNECTOR_CONFIG" >"$TMP_DIR/connector.log" 2>&1 &
CONNECTOR_PID="$!"

INSTANCES_JSON="$TMP_DIR/instances.json"
for _ in $(seq 1 20); do
  curl_json GET "$RELAY_URL/api/v2/instances" "$INSTANCES_JSON"
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

./bin/codencer-connectord status --config "$CONNECTOR_CONFIG" --json > "$TMP_DIR/connector-status.json"

if scenario_enabled status; then
  relay_cli connectors --json > "$TMP_DIR/relay-connectors.json"
  relay_cli instances --json > "$TMP_DIR/relay-instances.json"
fi

RUN_JSON="$TMP_DIR/run.json"
curl_json POST "$RELAY_URL/api/v2/instances/$INSTANCE_ID/runs" "$RUN_JSON" "{\"id\":\"$RUN_ID\",\"project_id\":\"$PROJECT_ID\"}"

STEP_JSON="$TMP_DIR/step.json"
curl_json POST "$RELAY_URL/api/v2/instances/$INSTANCE_ID/runs/$RUN_ID/steps" "$STEP_JSON" "{\"version\":\"v1\",\"goal\":\"Verify the self-host relay path\",\"adapter_profile\":\"$CONNECTOR_ADAPTER\",\"validations\":[{\"name\":\"bridge-build\",\"command\":\"go build ./...\"}]}"
STEP_ID="$(json_get "$STEP_JSON" '.id')"
if [[ -z "$STEP_ID" ]]; then
  echo "ERROR: failed to create step through relay" >&2
  cat "$STEP_JSON" >&2
  exit 1
fi

WAIT_JSON="$TMP_DIR/wait.json"
curl_json POST "$RELAY_URL/api/v2/steps/$STEP_ID/wait" "$WAIT_JSON" '{"timeout_ms":300000,"interval_ms":1000,"include_result":false}'

RESULT_JSON="$TMP_DIR/result.json"
curl_json GET "$RELAY_URL/api/v2/steps/$STEP_ID/result" "$RESULT_JSON"

VALIDATIONS_JSON="$TMP_DIR/validations.json"
curl_json GET "$RELAY_URL/api/v2/steps/$STEP_ID/validations" "$VALIDATIONS_JSON"

LOGS_FILE="$TMP_DIR/step-logs.txt"
curl_json GET "$RELAY_URL/api/v2/steps/$STEP_ID/logs" "$LOGS_FILE"

ARTIFACTS_JSON="$TMP_DIR/artifacts.json"
curl_json GET "$RELAY_URL/api/v2/steps/$STEP_ID/artifacts" "$ARTIFACTS_JSON"

GATES_JSON="$TMP_DIR/gates.json"
curl_json GET "$RELAY_URL/api/v2/instances/$INSTANCE_ID/runs/$RUN_ID/gates" "$GATES_JSON"

FIRST_ARTIFACT_ID="$(json_first_field "$ARTIFACTS_JSON" 'id')"
if [[ -n "$FIRST_ARTIFACT_ID" ]]; then
  curl_json GET "$RELAY_URL/api/v2/artifacts/$FIRST_ARTIFACT_ID/content" "$TMP_DIR/artifact-content.bin"
fi

if scenario_enabled mcp; then
  MCP_INIT_HEADERS="$TMP_DIR/mcp-init.headers"
  MCP_INIT_JSON="$TMP_DIR/mcp-init.json"
  curl -fsS -D "$MCP_INIT_HEADERS" -X POST "$RELAY_URL/mcp" \
    -H "Authorization: Bearer $PLANNER_TOKEN" \
    -H 'Content-Type: application/json' \
    -H 'MCP-Protocol-Version: 2025-11-25' \
    -d '{"jsonrpc":"2.0","id":"init-1","method":"initialize","params":{"protocolVersion":"2025-11-25"}}' \
    > "$MCP_INIT_JSON"
  MCP_SESSION_ID="$(awk 'BEGIN{IGNORECASE=1}/^MCP-Session-Id:/{print $2}' "$MCP_INIT_HEADERS" | tr -d '\r' | tail -n 1)"
  if [[ -n "$MCP_SESSION_ID" ]]; then
    curl -fsS "$RELAY_URL/mcp" \
      -H "Authorization: Bearer $PLANNER_TOKEN" \
      -H "MCP-Session-Id: $MCP_SESSION_ID" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      > "$TMP_DIR/mcp-stream.txt"
    curl -fsS -X POST "$RELAY_URL/mcp/call" \
      -H "Authorization: Bearer $PLANNER_TOKEN" \
      -H 'Content-Type: application/json' \
      -H "MCP-Session-Id: $MCP_SESSION_ID" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      -d '{"jsonrpc":"2.0","id":"tools-1","method":"tools/list","params":{}}' \
      > "$TMP_DIR/mcp-tools.json"
    curl -fsS -X POST "$RELAY_URL/mcp" \
      -H "Authorization: Bearer $PLANNER_TOKEN" \
      -H 'Content-Type: application/json' \
      -H "MCP-Session-Id: $MCP_SESSION_ID" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      -d "{\"jsonrpc\":\"2.0\",\"id\":\"call-1\",\"method\":\"tools/call\",\"params\":{\"name\":\"codencer.list_instances\",\"arguments\":{}}}" \
      > "$TMP_DIR/mcp-list-instances.json"
    curl -fsS -X POST "$RELAY_URL/mcp" \
      -H "Authorization: Bearer $PLANNER_TOKEN" \
      -H 'Content-Type: application/json' \
      -H "MCP-Session-Id: $MCP_SESSION_ID" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      -d "{\"jsonrpc\":\"2.0\",\"id\":\"call-2\",\"method\":\"tools/call\",\"params\":{\"name\":\"codencer.get_step_result\",\"arguments\":{\"step_id\":\"$STEP_ID\"}}}" \
      > "$TMP_DIR/mcp-step-result.json"
    curl -fsS -X POST "$RELAY_URL/mcp" \
      -H "Authorization: Bearer $PLANNER_TOKEN" \
      -H 'Content-Type: application/json' \
      -H "MCP-Session-Id: $MCP_SESSION_ID" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      -d "{\"jsonrpc\":\"2.0\",\"id\":\"call-3\",\"method\":\"tools/call\",\"params\":{\"name\":\"codencer.list_run_gates\",\"arguments\":{\"instance_id\":\"$INSTANCE_ID\",\"run_id\":\"$RUN_ID\"}}}" \
      > "$TMP_DIR/mcp-run-gates.json"
    curl -fsS -X POST "$RELAY_URL/mcp" \
      -H "Authorization: Bearer $PLANNER_TOKEN" \
      -H 'Content-Type: application/json' \
      -H "MCP-Session-Id: $MCP_SESSION_ID" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      -d "{\"jsonrpc\":\"2.0\",\"id\":\"call-4\",\"method\":\"tools/call\",\"params\":{\"name\":\"codencer.get_step_logs\",\"arguments\":{\"step_id\":\"$STEP_ID\"}}}" \
      > "$TMP_DIR/mcp-step-logs.json"
    curl -fsS -X DELETE "$RELAY_URL/mcp" \
      -H "Authorization: Bearer $PLANNER_TOKEN" \
      -H "MCP-Session-Id: $MCP_SESSION_ID" \
      -H 'MCP-Protocol-Version: 2025-11-25' \
      >/dev/null
  fi
fi

if scenario_enabled retry; then
  RETRY_JSON="$TMP_DIR/retry.json"
  retry_status="$(curl_best_effort POST "$RELAY_URL/api/v2/steps/$STEP_ID/retry" "$RETRY_JSON")"
  echo "Retry scenario status: $retry_status"
fi

if scenario_enabled gate; then
  GATE_ID="$(json_first_field "$GATES_JSON" 'id')"
  if [[ -n "$GATE_ID" ]]; then
    GATE_JSON="$TMP_DIR/gate-action.json"
    gate_status="$(curl_best_effort POST "$RELAY_URL/api/v2/gates/$GATE_ID/$GATE_ACTION" "$GATE_JSON")"
    echo "Gate scenario status: $gate_status ($GATE_ACTION $GATE_ID)"
  else
    echo "Gate scenario skipped: no gates were produced for $RUN_ID"
  fi
fi

if scenario_enabled abort; then
  ABORT_RUN_ID="${RUN_ID}-abort"
  ABORT_RUN_JSON="$TMP_DIR/abort-run.json"
  ABORT_STEP_JSON="$TMP_DIR/abort-step.json"
  ABORT_RESULT_JSON="$TMP_DIR/abort-result.json"
  curl_json POST "$RELAY_URL/api/v2/instances/$INSTANCE_ID/runs" "$ABORT_RUN_JSON" "{\"id\":\"$ABORT_RUN_ID\",\"project_id\":\"$PROJECT_ID\"}"
  curl_json POST "$RELAY_URL/api/v2/instances/$INSTANCE_ID/runs/$ABORT_RUN_ID/steps" "$ABORT_STEP_JSON" "{\"version\":\"v1\",\"goal\":\"Hold long enough for an abort request\",\"adapter_profile\":\"$CONNECTOR_ADAPTER\",\"timeout_seconds\":600}"
  abort_status="$(curl_best_effort POST "$RELAY_URL/api/v2/instances/$INSTANCE_ID/runs/$ABORT_RUN_ID/abort" "$ABORT_RESULT_JSON")"
  echo "Abort scenario status: $abort_status"
fi

if scenario_enabled audit; then
  curl_json GET "$RELAY_URL/api/v2/audit?limit=20" "$TMP_DIR/audit.json"
fi

echo "--- Self-Host Smoke Summary ---"
echo "Run:         $RUN_ID"
echo "Step:        $STEP_ID"
echo "State:       $(json_get "$WAIT_JSON" '.state')"
echo "Terminal:    $(json_get "$WAIT_JSON" '.terminal')"
echo "Summary:     $(json_get "$RESULT_JSON" '.summary')"
echo "Validations: $VALIDATIONS_JSON"
echo "Logs:        $LOGS_FILE"
echo "Artifacts:   $ARTIFACTS_JSON"
if [[ -n "$FIRST_ARTIFACT_ID" ]]; then
  echo "Artifact ID: $FIRST_ARTIFACT_ID"
fi
if scenario_enabled audit; then
  echo "Audit:       $TMP_DIR/audit.json"
fi
if scenario_enabled mcp; then
  echo "MCP:         $TMP_DIR/mcp-tools.json"
fi
