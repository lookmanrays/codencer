#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-official-connector.XXXXXX")"
DAEMON_PID=""
OFFICIAL_RELAY_PID=""
SELFHOST_RELAY_PID=""
GATEWAY_PID=""
OFFICIAL_CONNECTOR_PID=""
OFFICIAL_CONNECTOR2_PID=""
SELFHOST_CONNECTOR_PID=""

cleanup() {
  for pid in "$SELFHOST_CONNECTOR_PID" "$OFFICIAL_CONNECTOR2_PID" "$OFFICIAL_CONNECTOR_PID" "$GATEWAY_PID" "$SELFHOST_RELAY_PID" "$OFFICIAL_RELAY_PID" "$DAEMON_PID"; do
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
  for _ in $(seq 1 250); do
    if curl -fsS "$@" "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

json_get() {
  python3 - "$1" "$2" <<'PY'
import json, sys
value=json.load(open(sys.argv[1]))
for part in sys.argv[2].split("."):
  if part:
    value=value[part] if not part.isdigit() else value[int(part)]
print(value)
PY
}

repo="$TMPDIR_ROOT/repo"
home="$TMPDIR_ROOT/home"
home2="$TMPDIR_ROOT/home2"
gateway_home="$TMPDIR_ROOT/gateway-home"
state="$TMPDIR_ROOT/state"
mkdir -p "$repo" "$home" "$home2" "$gateway_home" "$state"

git -C "$repo" init -q
printf 'official connector verification\n' > "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" -c user.name=Codencer -c user.email=codencer@example.invalid commit -q -m "initial"

daemon_port="$(free_port)"
official_relay_port="$(free_port)"
selfhost_relay_port="$(free_port)"
gateway_port="$(free_port)"
daemon_url="http://127.0.0.1:$daemon_port"
official_relay_url="http://127.0.0.1:$official_relay_port"
selfhost_relay_url="http://127.0.0.1:$selfhost_relay_port"
gateway_url="http://127.0.0.1:$gateway_port"
official_token="official-relay-token"
selfhost_token="selfhost-relay-token"
gateway_token="gateway-dev-token"

echo "Official connector verification ports: daemon=$daemon_port official_relay=$official_relay_port selfhost_relay=$selfhost_relay_port gateway=$gateway_port"

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
  "$ROOT/bin/orchestratord" --config "$daemon_config" --repo-root "$repo" > "$TMPDIR_ROOT/daemon.log" 2>&1 &
DAEMON_PID="$!"
if ! wait_http "$daemon_url/health"; then
  echo "daemon did not become healthy; log follows" >&2
  cat "$TMPDIR_ROOT/daemon.log" >&2 || true
  exit 1
fi

init_project_home() {
  local local_home="$1"
  local label="$2"
  CODENCER_HOME="$local_home" "$ROOT/bin/codencer" init --json >/dev/null
  CODENCER_HOME="$local_home" "$ROOT/bin/codencer" machine set-label "$label" --json >/dev/null
  CODENCER_HOME="$local_home" "$ROOT/bin/codencer" project init \
    --id codencer \
    --repo "$repo" \
    --adapter fake \
    --profile fake-success \
    --daemon-url "$daemon_url" \
    --share-to-relay \
    --json >/dev/null
}

init_project_home "$home" official-one
init_project_home "$home2" official-two

start_relay() {
  local name="$1"
  local port="$2"
  local url="$3"
  local token="$4"
  local config="$TMPDIR_ROOT/$name-relay.json"
  cat > "$config" <<JSON
{
  "host": "127.0.0.1",
  "port": $port,
  "db_path": "$TMPDIR_ROOT/$name-relay.db",
  "planner_tokens": [
    {
      "name": "$name",
      "token": "$token",
      "scopes": ["*"]
    }
  ],
  "proxy_timeout_seconds": 90,
  "allowed_origins": ["$gateway_url"],
  "public_base_url": "$url"
}
JSON
  "$ROOT/bin/codencer-relayd" --config "$config" > "$TMPDIR_ROOT/$name-relay.log" 2>&1 &
  echo "$!:$config"
}

official_started="$(start_relay official "$official_relay_port" "$official_relay_url" "$official_token")"
OFFICIAL_RELAY_PID="${official_started%%:*}"
official_relay_config="${official_started#*:}"
selfhost_started="$(start_relay selfhost "$selfhost_relay_port" "$selfhost_relay_url" "$selfhost_token")"
SELFHOST_RELAY_PID="${selfhost_started%%:*}"
selfhost_relay_config="${selfhost_started#*:}"
_="$official_relay_config"
_="$selfhost_relay_config"

if ! wait_http "$official_relay_url/api/v2/status" -H "Authorization: Bearer $official_token"; then
  echo "official relay did not become healthy; log follows" >&2
  cat "$TMPDIR_ROOT/official-relay.log" >&2 || true
  exit 1
fi
if ! wait_http "$selfhost_relay_url/api/v2/status" -H "Authorization: Bearer $selfhost_token"; then
  echo "self-host relay did not become healthy; log follows" >&2
  cat "$TMPDIR_ROOT/selfhost-relay.log" >&2 || true
  exit 1
fi

CODENCER_HOME="$gateway_home" "$ROOT/bin/codencer" setup self-host \
  --gateway-url "$gateway_url" \
  --mcp-url "$gateway_url/mcp" \
  --relay-url "$official_relay_url" \
  --listen "127.0.0.1:$gateway_port" \
  --store "$TMPDIR_ROOT/gateway.db" \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --enable-oauth-dev \
  --oauth-client-secret gateway-oauth-secret \
  --json > "$TMPDIR_ROOT/setup-gateway.json"
gateway_config="$gateway_home/runtime/gateway/config.json"
test -f "$gateway_config"

export CODENCER_GATEWAY_MCP_TOKEN="$gateway_token"
export CODENCER_DEFAULT_RELAY_TOKEN="$official_token"
export CODENCER_SELFHOST_RELAY_TOKEN="$selfhost_token"
"$ROOT/bin/codencer-gatewayd" --config "$gateway_config" > "$TMPDIR_ROOT/gateway.log" 2>&1 &
GATEWAY_PID="$!"
if ! wait_http "$gateway_url/health"; then
  echo "gateway did not become healthy; log follows" >&2
  cat "$TMPDIR_ROOT/gateway.log" >&2 || true
  exit 1
fi

CODENCER_HOME="$home" "$ROOT/bin/codencer" login \
  --gateway "$gateway_url" \
  --email dev@example.com \
  --display-name "Dev User" \
  --dev-approve \
  --timeout 10s \
  --json > "$TMPDIR_ROOT/login.json"
if grep -q 'access_token\|official-relay-token\|selfhost-relay-token' "$TMPDIR_ROOT/login.json"; then
  echo "login output leaked token material" >&2
  cat "$TMPDIR_ROOT/login.json" >&2
  exit 1
fi
cp "$home/session.json" "$home2/session.json"
gateway_access_token="$(json_get "$home/session.json" "access_token")"
test -n "$gateway_access_token"

CODENCER_HOME="$home" "$ROOT/bin/codencer" connector login \
  --gateway "$gateway_url" \
  --relay default \
  --daemon-url "$daemon_url" \
  --config "$TMPDIR_ROOT/official-connector.json" \
  --json > "$TMPDIR_ROOT/official-connector-login.json"
if grep -q 'enroll-secret\|official-relay-token\|private_key' "$TMPDIR_ROOT/official-connector-login.json"; then
  echo "official connector login output leaked secret material" >&2
  cat "$TMPDIR_ROOT/official-connector-login.json" >&2
  exit 1
fi
"$ROOT/bin/codencer-connectord" run --config "$TMPDIR_ROOT/official-connector.json" > "$TMPDIR_ROOT/official-connector.log" 2>&1 &
OFFICIAL_CONNECTOR_PID="$!"

CODENCER_HOME="$home2" "$ROOT/bin/codencer" connector login \
  --gateway "$gateway_url" \
  --relay default \
  --daemon-url "$daemon_url" \
  --config "$TMPDIR_ROOT/official-connector-2.json" \
  --json > "$TMPDIR_ROOT/official-connector-2-login.json"
"$ROOT/bin/codencer-connectord" run --config "$TMPDIR_ROOT/official-connector-2.json" > "$TMPDIR_ROOT/official-connector-2.log" 2>&1 &
OFFICIAL_CONNECTOR2_PID="$!"

CODENCER_HOME="$home" "$ROOT/bin/codencer" gateway relay add \
  --gateway "$gateway_url" \
  --name personal \
  --url "$selfhost_relay_url" \
  --token-env CODENCER_SELFHOST_RELAY_TOKEN \
  --json > "$TMPDIR_ROOT/relay-add.json"
if grep -q 'selfhost-relay-token\|official-relay-token' "$TMPDIR_ROOT/relay-add.json"; then
  echo "relay add output leaked token material" >&2
  cat "$TMPDIR_ROOT/relay-add.json" >&2
  exit 1
fi

CODENCER_HOME="$home" "$ROOT/bin/codencer" connector login \
  --gateway "$gateway_url" \
  --relay personal \
  --daemon-url "$daemon_url" \
  --config "$TMPDIR_ROOT/selfhost-connector.json" \
  --json > "$TMPDIR_ROOT/selfhost-connector-login.json"
"$ROOT/bin/codencer-connectord" run --config "$TMPDIR_ROOT/selfhost-connector.json" > "$TMPDIR_ROOT/selfhost-connector.log" 2>&1 &
SELFHOST_CONNECTOR_PID="$!"

wait_projects() {
  local relay_url="$1"
  local token="$2"
  local out="$3"
  local min_locations="$4"
  for _ in $(seq 1 250); do
    curl -fsS -H "Authorization: Bearer $token" "$relay_url/api/v2/projects" > "$out" || true
    locations_count="$(python3 - "$out" <<'PY'
import json, sys
try:
  projects=json.load(open(sys.argv[1]))
  print(len(projects[0].get("locations", [])))
except Exception:
  print(0)
PY
)"
    if grep -q '"project_id":"codencer"\|"project_id": "codencer"' "$out" && [ "$locations_count" -ge "$min_locations" ]; then
      return 0
    fi
    sleep 0.1
  done
  cat "$out" >&2 || true
  return 1
}

wait_projects "$official_relay_url" "$official_token" "$TMPDIR_ROOT/official-projects.json" 2
wait_projects "$selfhost_relay_url" "$selfhost_token" "$TMPDIR_ROOT/selfhost-projects.json" 1

for out in "$TMPDIR_ROOT/official-projects.json" "$TMPDIR_ROOT/selfhost-projects.json"; do
  if grep -q "$repo" "$out"; then
    echo "relay project listing leaked absolute repo path" >&2
    cat "$out" >&2
    exit 1
  fi
done

official_machine_id="$(json_get "$TMPDIR_ROOT/official-projects.json" "0.locations.0.machine_id")"
selfhost_machine_id="$(json_get "$TMPDIR_ROOT/selfhost-projects.json" "0.locations.0.machine_id")"
test -n "$official_machine_id"
test -n "$selfhost_machine_id"

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
      -H "Authorization: Bearer $gateway_access_token" \
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
      -H "Authorization: Bearer $gateway_access_token" \
      -H "Accept: application/json, text/event-stream" \
      -H "Content-Type: application/json" \
      -H "MCP-Protocol-Version: 2025-11-25" \
      --data "$payload" \
      "$gateway_url/mcp"
  fi
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

mcp_post '{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"verify-official-connector","version":"v0"}}}' "$TMPDIR_ROOT/mcp-init.json"
test -n "$SESSION_ID"
grep -q '"serverInfo"' "$TMPDIR_ROOT/mcp-init.json" || { cat "$TMPDIR_ROOT/mcp-init.json" >&2; exit 1; }

mcp_post '{"jsonrpc":"2.0","id":"tools","method":"tools/list","params":{}}' "$TMPDIR_ROOT/mcp-tools.json"
for tool in codencer.list_relays codencer.list_projects codencer.run_project_manifest codencer.get_run_report codencer.get_blocker; do
  grep -q "$tool" "$TMPDIR_ROOT/mcp-tools.json" || { cat "$TMPDIR_ROOT/mcp-tools.json" >&2; exit 1; }
done

mcp_tool "codencer.list_relays" '{}' "$TMPDIR_ROOT/mcp-list-relays.json"
grep -q '"default"\|"personal"' "$TMPDIR_ROOT/mcp-list-relays.json" || { cat "$TMPDIR_ROOT/mcp-list-relays.json" >&2; exit 1; }

mcp_tool "codencer.list_projects" '{}' "$TMPDIR_ROOT/mcp-list-projects.json"
grep -q '"project_id":"codencer"\|"project_id": "codencer"' "$TMPDIR_ROOT/mcp-list-projects.json" || { cat "$TMPDIR_ROOT/mcp-list-projects.json" >&2; exit 1; }

python3 - "$ROOT/testdata/manifests/fake-success.yaml" "$official_machine_id" > "$TMPDIR_ROOT/run-default-args.json" <<'PY'
import json, sys
manifest=open(sys.argv[1]).read()
machine_id=sys.argv[2]
print(json.dumps({"relay_profile_id":"default","project_id":"codencer","machine_id":machine_id,"manifest_text":manifest,"manifest_name":"fake-success.yaml","wait":True}))
PY
mcp_tool "codencer.run_project_manifest" "$(cat "$TMPDIR_ROOT/run-default-args.json")" "$TMPDIR_ROOT/mcp-run-default.json"
grep -q '"ok":true\|"ok": true' "$TMPDIR_ROOT/mcp-run-default.json" || { cat "$TMPDIR_ROOT/mcp-run-default.json" >&2; exit 1; }
run_id="$(python3 - "$TMPDIR_ROOT/mcp-run-default.json" <<'PY'
import json, sys
payload=json.load(open(sys.argv[1]))
sc=payload.get("result", {}).get("structuredContent", {})
print(sc.get("run", {}).get("id", "") or sc.get("run_id", ""))
PY
)"
test -n "$run_id"

mcp_tool "codencer.get_run_report" "{\"relay_profile_id\":\"default\",\"project_id\":\"codencer\",\"machine_id\":\"$official_machine_id\",\"run_id\":\"$run_id\"}" "$TMPDIR_ROOT/mcp-run-report.json"
grep -q "$run_id" "$TMPDIR_ROOT/mcp-run-report.json" || { cat "$TMPDIR_ROOT/mcp-run-report.json" >&2; exit 1; }

python3 - "$ROOT/testdata/manifests/fake-success.yaml" "$selfhost_machine_id" > "$TMPDIR_ROOT/run-selfhost-args.json" <<'PY'
import json, sys
manifest=open(sys.argv[1]).read()
machine_id=sys.argv[2]
print(json.dumps({"relay_profile_id":"personal","project_id":"codencer","machine_id":machine_id,"manifest_text":manifest,"manifest_name":"fake-success.yaml","wait":True}))
PY
mcp_tool "codencer.run_project_manifest" "$(cat "$TMPDIR_ROOT/run-selfhost-args.json")" "$TMPDIR_ROOT/mcp-run-selfhost.json"
grep -q '"ok":true\|"ok": true' "$TMPDIR_ROOT/mcp-run-selfhost.json" || { cat "$TMPDIR_ROOT/mcp-run-selfhost.json" >&2; exit 1; }

mcp_tool "codencer.run_project_manifest" "{\"project_id\":\"codencer\",\"manifest_text\":\"version: v0.3\\nkind: codencer.run_plan\\n\",\"wait\":true}" "$TMPDIR_ROOT/mcp-ambiguous-relay.json"
grep -q 'ambiguous_relay_profile' "$TMPDIR_ROOT/mcp-ambiguous-relay.json" || { cat "$TMPDIR_ROOT/mcp-ambiguous-relay.json" >&2; exit 1; }

mcp_tool "codencer.run_project_manifest" "{\"relay_profile_id\":\"default\",\"project_id\":\"codencer\",\"manifest_text\":\"version: v0.3\\nkind: codencer.run_plan\\n\",\"wait\":true}" "$TMPDIR_ROOT/mcp-ambiguous-location.json"
grep -q 'ambiguous_project_location' "$TMPDIR_ROOT/mcp-ambiguous-location.json" || { cat "$TMPDIR_ROOT/mcp-ambiguous-location.json" >&2; exit 1; }

kill "$SELFHOST_RELAY_PID" 2>/dev/null || true
wait "$SELFHOST_RELAY_PID" 2>/dev/null || true
SELFHOST_RELAY_PID=""
mcp_tool "codencer.run_project_manifest" "{\"relay_profile_id\":\"personal\",\"project_id\":\"codencer\",\"machine_id\":\"$selfhost_machine_id\",\"manifest_text\":\"version: v0.3\\nkind: codencer.run_plan\\n\",\"wait\":true}" "$TMPDIR_ROOT/mcp-relay-down.json"
grep -q 'relay_unavailable' "$TMPDIR_ROOT/mcp-relay-down.json" || { cat "$TMPDIR_ROOT/mcp-relay-down.json" >&2; exit 1; }

for out in "$TMPDIR_ROOT"/mcp-*.json; do
  if grep -q "$repo\|$official_token\|$selfhost_token\|$gateway_token\|$gateway_access_token" "$out"; then
    echo "MCP output leaked path or token: $out" >&2
    cat "$out" >&2
    exit 1
  fi
done

echo "--- Official Connector E2E Summary ---"
echo "Gateway:          $gateway_url"
echo "Official Relay:   $official_relay_url"
echo "Self-host Relay:  $selfhost_relay_url"
echo "Daemon:           $daemon_url"
echo "Run:              $run_id"
echo "Official machine: $official_machine_id"
echo "Self-host machine:$selfhost_machine_id"
