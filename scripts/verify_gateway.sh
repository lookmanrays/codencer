#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${CODENCER_BIN_DIR:-$ROOT/bin}"
MANIFEST_FILE="${CODENCER_MANIFEST_FILE:-$ROOT/testdata/manifests/fake-success.yaml}"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-gateway.XXXXXX")"
DAEMON_PID=""
RELAY_PID=""
CONNECTOR_PID=""
CONNECTOR2_PID=""
GATEWAY_PID=""

cleanup() {
  for pid in "$GATEWAY_PID" "$CONNECTOR2_PID" "$CONNECTOR_PID" "$RELAY_PID" "$DAEMON_PID"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  rm -rf "$TMPDIR_ROOT"
}
trap cleanup EXIT

free_port() {
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

wait_http() {
  local url="$1"
  shift
  for _ in $(seq 1 200); do
    if curl -fsS "$@" "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

print_json() {
  if command -v jq >/dev/null 2>&1; then
    jq .
  else
    cat
  fi
}

assert_no_mcp_leaks() {
  local file="$1"
  if grep -Eq '/Users/|/tmp/|/var/folders/|CODENCER_HOME|\.codencer-live-test|report_path|logs_ref|normalized_task_ref|original_input_ref|"path"' "$file"; then
    echo "MCP output leaked a local path or unsafe path field: $file" >&2
    cat "$file" >&2
    exit 1
  fi
}

repo="$TMPDIR_ROOT/repo"
home="$TMPDIR_ROOT/home"
home2="$TMPDIR_ROOT/home2"
state="$TMPDIR_ROOT/state"
mkdir -p "$repo" "$home" "$home2" "$state"

git -C "$repo" init -q
printf 'gateway verification\n' > "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" -c user.name=Codencer -c user.email=codencer@example.invalid commit -q -m "initial"

daemon_port="$(free_port)"
relay_port="$(free_port)"
gateway_port="$(free_port)"
daemon_url="http://127.0.0.1:$daemon_port"
relay_url="http://127.0.0.1:$relay_port"
gateway_url="http://127.0.0.1:$gateway_port"
planner_token="planner-token"
gateway_token="gateway-token"

echo "Gateway verification ports: daemon=$daemon_port relay=$relay_port gateway=$gateway_port"

daemon_config="$TMPDIR_ROOT/daemon.json"
cat > "$daemon_config" <<JSON
{
  "log_level": "error",
  "db_path": "$state/codencer.db",
  "artifact_root": "$state/artifacts",
  "workspace_root": "$state/workspace",
  "repo_root": "$repo",
  "host": "127.0.0.1",
  "port": $daemon_port
}
JSON

env \
  PORT="$daemon_port" \
  HOST="127.0.0.1" \
  DB_PATH="$state/codencer.db" \
  ARTIFACT_ROOT="$state/artifacts" \
  WORKSPACE_ROOT="$state/workspace" \
  LOG_LEVEL="error" \
  REPO_ROOT="$repo" \
  "$BIN_DIR/orchestratord" --config "$daemon_config" --repo-root "$repo" > "$TMPDIR_ROOT/daemon.log" 2>&1 &
DAEMON_PID="$!"
if ! wait_http "$daemon_url/health"; then
  echo "daemon did not become healthy; log follows" >&2
  cat "$TMPDIR_ROOT/daemon.log" >&2 || true
  exit 1
fi

export CODENCER_HOME="$home"
"$BIN_DIR/codencer" init --json >/dev/null
"$BIN_DIR/codencer" machine set-label gateway-host --json >/dev/null
"$BIN_DIR/codencer" project init \
  --id codencer \
  --repo "$repo" \
  --adapter fake \
  --profile fake-success \
  --daemon-url "$daemon_url" \
  --share-to-relay \
  --json >/dev/null

CODENCER_HOME="$home2" "$BIN_DIR/codencer" init --json >/dev/null
CODENCER_HOME="$home2" "$BIN_DIR/codencer" machine set-label gateway-host-2 --json >/dev/null
CODENCER_HOME="$home2" "$BIN_DIR/codencer" project init \
  --id codencer \
  --repo "$repo" \
  --adapter fake \
  --profile fake-success \
  --daemon-url "$daemon_url" \
  --share-to-relay \
  --json >/dev/null

relay_config="$TMPDIR_ROOT/relay.json"
cat > "$relay_config" <<JSON
{
  "host": "127.0.0.1",
  "port": $relay_port,
  "db_path": "$TMPDIR_ROOT/relay.db",
  "planner_tokens": [
    {
      "name": "operator",
      "token": "$planner_token",
      "scopes": ["*"]
    }
  ],
  "proxy_timeout_seconds": 90,
  "allowed_origins": ["http://127.0.0.1:$gateway_port"],
  "public_base_url": "$relay_url"
}
JSON
"$BIN_DIR/codencer-relayd" --config "$relay_config" > "$TMPDIR_ROOT/relay.log" 2>&1 &
RELAY_PID="$!"
if ! wait_http "$relay_url/api/v2/status" -H "Authorization: Bearer $planner_token"; then
  echo "relay did not become healthy; log follows" >&2
  cat "$TMPDIR_ROOT/relay.log" >&2 || true
  exit 1
fi

enroll_connector() {
  local label="$1"
  local config="$2"
  local output="$3"
  local connector_home="$4"
  local enrollment_json="$TMPDIR_ROOT/enrollment-$label.json"
  "$BIN_DIR/codencer-relayd" enrollment-token create \
    --config "$relay_config" \
    --label "$label" \
    --expires-in-seconds 600 \
    --json > "$enrollment_json"
  local enrollment_secret
  enrollment_secret="$(python3 - "$enrollment_json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))["secret"])
PY
)"
  "$BIN_DIR/codencer-connectord" enroll \
    --relay-url "$relay_url" \
    --daemon-url "$daemon_url" \
    --enrollment-token "$enrollment_secret" \
    --config "$config" \
    --codencer-home "$connector_home" \
    --label "$label" >/dev/null
  "$BIN_DIR/codencer-connectord" run --config "$config" > "$output" 2>&1 &
  echo "$!"
}

connector_config="$TMPDIR_ROOT/connector.json"
connector2_config="$TMPDIR_ROOT/connector2.json"
CONNECTOR_PID="$(enroll_connector gateway-one "$connector_config" "$TMPDIR_ROOT/connector.log" "$home")"
CONNECTOR2_PID="$(enroll_connector gateway-two "$connector2_config" "$TMPDIR_ROOT/connector2.log" "$home2")"

projects_json="$TMPDIR_ROOT/relay-projects.json"
for _ in $(seq 1 200); do
  curl -fsS -H "Authorization: Bearer $planner_token" "$relay_url/api/v2/projects" > "$projects_json" || true
  locations_count="$(python3 - "$projects_json" <<'PY'
import json, sys
try:
  projects=json.load(open(sys.argv[1]))
  print(len(projects[0].get("locations", [])))
except Exception:
  print(0)
PY
)"
  if grep -q '"project_id":"codencer"\|"project_id": "codencer"' "$projects_json" && [ "$locations_count" -ge 2 ]; then
    break
  fi
  sleep 0.1
done
grep -q '"project_id":"codencer"\|"project_id": "codencer"' "$projects_json" || { cat "$projects_json" >&2; exit 1; }
locations_count="$(python3 - "$projects_json" <<'PY'
import json, sys
projects=json.load(open(sys.argv[1]))
print(len(projects[0].get("locations", [])))
PY
)"
if [ "$locations_count" -lt 2 ]; then
  echo "expected two connector locations for ambiguity check" >&2
  cat "$projects_json" >&2
  exit 1
fi
if grep -q "$repo" "$projects_json"; then
  echo "relay project listing leaked absolute repo path" >&2
  cat "$projects_json" >&2
  exit 1
fi

machine_id="$(python3 - "$projects_json" <<'PY'
import json, sys
projects=json.load(open(sys.argv[1]))
print(projects[0]["locations"][0]["machine_id"])
PY
)"
host_label="$(python3 - "$projects_json" <<'PY'
import json, sys
projects=json.load(open(sys.argv[1]))
print(projects[0]["locations"][0]["host_label"])
PY
)"
test -n "$machine_id"
test -n "$host_label"

gateway_config="$TMPDIR_ROOT/gateway.json"
cat > "$gateway_config" <<JSON
{
  "version": 1,
  "public_base_url": "$gateway_url",
  "mcp_url": "$gateway_url/mcp",
  "listen_addr": "127.0.0.1:$gateway_port",
  "auth": {
    "mode": "bearer-dev",
    "token_env": "CODENCER_GATEWAY_MCP_TOKEN"
  },
  "oauth_dev": {
    "enabled": true,
    "issuer": "$gateway_url",
    "client_id": "codencer-chatgpt-dev"
  },
  "relay_profiles": [
    {
      "id": "personal",
      "name": "Personal self-host Relay",
      "url": "$relay_url",
      "token_env": "CODENCER_RELAY_PERSONAL_TOKEN",
      "enabled": true
    },
    {
      "id": "backup",
      "name": "Backup profile to same Relay",
      "url": "$relay_url",
      "token_env": "CODENCER_RELAY_PERSONAL_TOKEN",
      "enabled": true
    }
  ]
}
JSON

export CODENCER_GATEWAY_MCP_TOKEN="$gateway_token"
export CODENCER_RELAY_PERSONAL_TOKEN="$planner_token"
"$BIN_DIR/codencer-gatewayd" --config "$gateway_config" > "$TMPDIR_ROOT/gateway.log" 2>&1 &
GATEWAY_PID="$!"
if ! wait_http "$gateway_url/health"; then
  echo "gateway did not become healthy; log follows" >&2
  cat "$TMPDIR_ROOT/gateway.log" >&2 || true
  exit 1
fi

curl -fsS -H "Accept: application/json" "$gateway_url/.well-known/oauth-protected-resource/mcp" > "$TMPDIR_ROOT/gateway-protected-resource.json"
grep -q "$gateway_url/mcp" "$TMPDIR_ROOT/gateway-protected-resource.json" || { cat "$TMPDIR_ROOT/gateway-protected-resource.json" >&2; exit 1; }

tmp_headers="$TMPDIR_ROOT/mcp.headers"
tmp_body="$TMPDIR_ROOT/mcp.body"
SESSION_ID=""

mcp_post() {
  local payload="$1"
  local out="$2"
  if [ -n "$SESSION_ID" ]; then
    curl -fsS \
      -D "$tmp_headers" \
      -o "$tmp_body" \
      -H "Authorization: Bearer $gateway_token" \
      -H "Accept: application/json, text/event-stream" \
      -H "Content-Type: application/json" \
      -H "MCP-Protocol-Version: 2025-11-25" \
      -H "MCP-Session-Id: $SESSION_ID" \
      --data "$payload" \
      "$gateway_url/mcp"
  else
    curl -fsS \
      -D "$tmp_headers" \
      -o "$tmp_body" \
      -H "Authorization: Bearer $gateway_token" \
      -H "Accept: application/json, text/event-stream" \
      -H "Content-Type: application/json" \
      -H "MCP-Protocol-Version: 2025-11-25" \
      --data "$payload" \
      "$gateway_url/mcp"
  fi
  local returned_session
  returned_session="$(awk -F': ' 'tolower($1)=="mcp-session-id" {gsub("\r","",$2); print $2}' "$tmp_headers" | tail -n 1)"
  if [ -n "$returned_session" ]; then
    SESSION_ID="$returned_session"
  fi
  cp "$tmp_body" "$out"
}

mcp_tool() {
  local name="$1"
  local args_json="$2"
  local out="$3"
  python3 - "$name" "$args_json" > "$TMPDIR_ROOT/mcp-tool-request.json" <<'PY'
import json, sys
name=sys.argv[1]
args=json.loads(sys.argv[2])
print(json.dumps({"jsonrpc":"2.0","id":name,"method":"tools/call","params":{"name":name,"arguments":args}}))
PY
  mcp_post "$(cat "$TMPDIR_ROOT/mcp-tool-request.json")" "$out"
}

mcp_post '{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"verify-gateway","version":"v0"}}}' "$TMPDIR_ROOT/mcp-init.json"
test -n "$SESSION_ID"
grep -q '"serverInfo"' "$TMPDIR_ROOT/mcp-init.json" || { cat "$TMPDIR_ROOT/mcp-init.json" >&2; exit 1; }

mcp_post '{"jsonrpc":"2.0","id":"tools","method":"tools/list","params":{}}' "$TMPDIR_ROOT/mcp-tools.json"
for tool in codencer.list_relays codencer.list_projects codencer.run_project_manifest codencer.get_run_report codencer.get_blocker; do
  grep -q "$tool" "$TMPDIR_ROOT/mcp-tools.json" || { cat "$TMPDIR_ROOT/mcp-tools.json" >&2; exit 1; }
done

mcp_tool "codencer.list_relays" '{}' "$TMPDIR_ROOT/list-relays.json"
grep -q '"personal"' "$TMPDIR_ROOT/list-relays.json" || { cat "$TMPDIR_ROOT/list-relays.json" >&2; exit 1; }
assert_no_mcp_leaks "$TMPDIR_ROOT/list-relays.json"
if grep -q "$planner_token\|$gateway_token" "$TMPDIR_ROOT/list-relays.json"; then
  echo "gateway list_relays leaked a token" >&2
  cat "$TMPDIR_ROOT/list-relays.json" >&2
  exit 1
fi

mcp_tool "codencer.list_projects" '{}' "$TMPDIR_ROOT/list-projects.json"
grep -q '"project_id":"codencer"\|"project_id": "codencer"' "$TMPDIR_ROOT/list-projects.json" || { cat "$TMPDIR_ROOT/list-projects.json" >&2; exit 1; }
assert_no_mcp_leaks "$TMPDIR_ROOT/list-projects.json"
if grep -q "$repo\|$planner_token\|$gateway_token" "$TMPDIR_ROOT/list-projects.json"; then
  echo "gateway list_projects leaked local path or token" >&2
  cat "$TMPDIR_ROOT/list-projects.json" >&2
  exit 1
fi

python3 - "$MANIFEST_FILE" "$machine_id" > "$TMPDIR_ROOT/run-args.json" <<'PY'
import json, sys
manifest=open(sys.argv[1]).read()
machine_id=sys.argv[2]
print(json.dumps({"relay_profile_id":"personal","project_id":"codencer","machine_id":machine_id,"manifest_text":manifest,"manifest_name":"fake-success.yaml","wait":True}))
PY
mcp_tool "codencer.run_project_manifest" "$(cat "$TMPDIR_ROOT/run-args.json")" "$TMPDIR_ROOT/run-through-gateway.json"
grep -q '"ok":true\|"ok": true' "$TMPDIR_ROOT/run-through-gateway.json" || { cat "$TMPDIR_ROOT/run-through-gateway.json" >&2; exit 1; }
assert_no_mcp_leaks "$TMPDIR_ROOT/run-through-gateway.json"
run_id="$(python3 - "$TMPDIR_ROOT/run-through-gateway.json" <<'PY'
import json, sys
payload=json.load(open(sys.argv[1]))
sc=payload.get("result", {}).get("structuredContent", {})
print(sc.get("run", {}).get("id", "") or sc.get("run_id", ""))
PY
)"
test -n "$run_id"

mcp_tool "codencer.get_run_report" "{\"relay_profile_id\":\"personal\",\"project_id\":\"codencer\",\"machine_id\":\"$machine_id\",\"run_id\":\"$run_id\"}" "$TMPDIR_ROOT/run-report.json"
grep -q "$run_id" "$TMPDIR_ROOT/run-report.json" || { cat "$TMPDIR_ROOT/run-report.json" >&2; exit 1; }
assert_no_mcp_leaks "$TMPDIR_ROOT/run-report.json"

mcp_tool "codencer.run_project_manifest" "{\"project_id\":\"codencer\",\"manifest_text\":\"version: v0.3\\nkind: codencer.run_plan\\n\",\"wait\":true}" "$TMPDIR_ROOT/ambiguous-relay.json"
grep -q 'ambiguous_relay_profile' "$TMPDIR_ROOT/ambiguous-relay.json" || { cat "$TMPDIR_ROOT/ambiguous-relay.json" >&2; exit 1; }
assert_no_mcp_leaks "$TMPDIR_ROOT/ambiguous-relay.json"

mcp_tool "codencer.run_project_manifest" "{\"relay_profile_id\":\"personal\",\"project_id\":\"codencer\",\"manifest_text\":\"version: v0.3\\nkind: codencer.run_plan\\n\",\"wait\":true}" "$TMPDIR_ROOT/ambiguous-location.json"
grep -q 'ambiguous_project_location' "$TMPDIR_ROOT/ambiguous-location.json" || { cat "$TMPDIR_ROOT/ambiguous-location.json" >&2; exit 1; }
assert_no_mcp_leaks "$TMPDIR_ROOT/ambiguous-location.json"

kill "$RELAY_PID" 2>/dev/null || true
wait "$RELAY_PID" 2>/dev/null || true
RELAY_PID=""
mcp_tool "codencer.run_project_manifest" "{\"relay_profile_id\":\"personal\",\"project_id\":\"codencer\",\"machine_id\":\"$machine_id\",\"manifest_text\":\"version: v0.3\\nkind: codencer.run_plan\\n\",\"wait\":true}" "$TMPDIR_ROOT/relay-down.json"
grep -q 'relay_unavailable' "$TMPDIR_ROOT/relay-down.json" || { cat "$TMPDIR_ROOT/relay-down.json" >&2; exit 1; }
assert_no_mcp_leaks "$TMPDIR_ROOT/relay-down.json"

echo "--- Gateway Smoke Summary ---"
echo "Daemon:  $daemon_url"
echo "Relay:   $relay_url"
echo "Gateway: $gateway_url"
echo "Run:     $run_id"
echo "Machine: $machine_id"
echo "Host:    $host_label"
echo "MCP:     $TMPDIR_ROOT/mcp-tools.json"
