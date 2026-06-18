#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-local-relay-mcp.XXXXXX")"
DAEMON_PID=""
RELAY_PID=""
CONNECTOR_PID=""
TLS_RELAY_PID=""

cleanup() {
  for pid in "$CONNECTOR_PID" "$RELAY_PID" "$DAEMON_PID" "$TLS_RELAY_PID"; do
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
  for _ in $(seq 1 150); do
    if curl -fsS "$@" "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

repo="$TMPDIR_ROOT/repo"
home="$TMPDIR_ROOT/home"
state="$TMPDIR_ROOT/state"
mkdir -p "$repo" "$home" "$state"

git -C "$repo" init -q
printf 'local relay mcp verification\n' > "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" -c user.name=Codencer -c user.email=codencer@example.invalid commit -q -m "initial"

daemon_port="$(free_port)"
relay_port="$(free_port)"
tls_port="$(free_port)"
daemon_url="http://127.0.0.1:$daemon_port"
relay_url="http://127.0.0.1:$relay_port"
tls_url="https://127.0.0.1:$tls_port"
planner_token="planner-token"

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

export CODENCER_HOME="$home"
"$ROOT/bin/codencer" init --json >/dev/null
"$ROOT/bin/codencer" project init \
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
  "proxy_timeout_seconds": 60,
  "allowed_origins": ["http://127.0.0.1:$relay_port"],
  "public_base_url": "$relay_url"
}
JSON
"$ROOT/bin/codencer-relayd" --config "$relay_config" > "$TMPDIR_ROOT/relay.log" 2>&1 &
RELAY_PID="$!"
if ! wait_http "$relay_url/api/v2/status" -H "Authorization: Bearer $planner_token"; then
  echo "relay did not become healthy; log follows" >&2
  cat "$TMPDIR_ROOT/relay.log" >&2 || true
  exit 1
fi

if command -v openssl >/dev/null 2>&1; then
  cert="$TMPDIR_ROOT/relay.crt"
  key="$TMPDIR_ROOT/relay.key"
  openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
    -subj "/CN=127.0.0.1" \
    -addext "subjectAltName=IP:127.0.0.1" \
    -keyout "$key" -out "$cert" >/dev/null 2>&1
  tls_config="$TMPDIR_ROOT/relay-tls.json"
  cat > "$tls_config" <<JSON
{
  "host": "127.0.0.1",
  "port": $tls_port,
  "db_path": "$TMPDIR_ROOT/relay-tls.db",
  "planner_tokens": [{"name":"operator","token":"$planner_token","scopes":["admin:read"]}],
  "tls_cert_file": "$cert",
  "tls_key_file": "$key",
  "public_base_url": "$tls_url"
}
JSON
  "$ROOT/bin/codencer-relayd" --config "$tls_config" > "$TMPDIR_ROOT/relay-tls.log" 2>&1 &
  TLS_RELAY_PID="$!"
  if ! wait_http "$tls_url/api/v2/status" -k -H "Authorization: Bearer $planner_token"; then
    echo "TLS relay did not become healthy; log follows" >&2
    cat "$TMPDIR_ROOT/relay-tls.log" >&2 || true
    exit 1
  fi
fi

enrollment_json="$TMPDIR_ROOT/enrollment.json"
"$ROOT/bin/codencer-relayd" enrollment-token create \
  --config "$relay_config" \
  --label local-relay-mcp \
  --expires-in-seconds 600 \
  --json > "$enrollment_json"
enrollment_secret="$(python3 - "$enrollment_json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))["secret"])
PY
)"

connector_config="$TMPDIR_ROOT/connector.json"
"$ROOT/bin/codencer-connectord" enroll \
  --relay-url "$relay_url" \
  --daemon-url "$daemon_url" \
  --enrollment-token "$enrollment_secret" \
  --config "$connector_config" \
  --codencer-home "$home" >/dev/null
"$ROOT/bin/codencer-connectord" run --config "$connector_config" > "$TMPDIR_ROOT/connector.log" 2>&1 &
CONNECTOR_PID="$!"

projects_json="$TMPDIR_ROOT/projects.json"
for _ in $(seq 1 150); do
  curl -fsS -H "Authorization: Bearer $planner_token" "$relay_url/api/v2/projects" > "$projects_json" || true
  if grep -q '"project_id":"codencer"\|"project_id": "codencer"' "$projects_json"; then
    break
  fi
  sleep 0.1
done
grep -q '"project_id":"codencer"\|"project_id": "codencer"' "$projects_json" || { cat "$projects_json" >&2; exit 1; }
grep -q '"locations"' "$projects_json" || { cat "$projects_json" >&2; exit 1; }
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

curl -fsS \
  -H "Authorization: Bearer $planner_token" \
  -H "Content-Type: application/json" \
  -d '{"goal":"relay machine selector fake success","profile":"fake-success","wait":true}' \
  "$relay_url/api/v2/projects/codencer/submit?machine_id=$machine_id" > "$TMPDIR_ROOT/submit-machine.json"
grep -q '"ok":true\|"ok": true' "$TMPDIR_ROOT/submit-machine.json" || { cat "$TMPDIR_ROOT/submit-machine.json" >&2; exit 1; }

curl -fsS \
  -H "Authorization: Bearer $planner_token" \
  "$relay_url/api/v2/projects/codencer/runs?host_label=$host_label" > "$TMPDIR_ROOT/runs-host-label.json"
grep -q '"project_id":"codencer"\|"project_id": "codencer"\|\[\]' "$TMPDIR_ROOT/runs-host-label.json" || { cat "$TMPDIR_ROOT/runs-host-label.json" >&2; exit 1; }

curl -fsS \
  -H "Authorization: Bearer $planner_token" \
  -H "Content-Type: application/json" \
  -d '{"goal":"relay fake success","profile":"fake-success","wait":true}' \
  "$relay_url/api/v2/projects/codencer/submit" > "$TMPDIR_ROOT/submit.json"
grep -q '"ok":true\|"ok": true' "$TMPDIR_ROOT/submit.json" || { cat "$TMPDIR_ROOT/submit.json" >&2; exit 1; }

step_id="$(python3 - "$TMPDIR_ROOT/submit.json" <<'PY'
import json, sys
payload=json.load(open(sys.argv[1]))
print(payload.get("task", {}).get("step_id", ""))
PY
)"
test -n "$step_id"
curl -fsS -H "Authorization: Bearer $planner_token" \
  "$relay_url/api/v2/projects/codencer/steps/$step_id/result" > "$TMPDIR_ROOT/step-result.json"
grep -q '"completed"' "$TMPDIR_ROOT/step-result.json" || { cat "$TMPDIR_ROOT/step-result.json" >&2; exit 1; }

python3 - "$ROOT/testdata/manifests/fake-success.yaml" > "$TMPDIR_ROOT/run-plan-request.json" <<'PY'
import json, sys
print(json.dumps({"manifest_text": open(sys.argv[1]).read(), "manifest_name": "fake-success.yaml", "wait": True}))
PY
curl -fsS \
  -H "Authorization: Bearer $planner_token" \
  -H "Content-Type: application/json" \
  -d @"$TMPDIR_ROOT/run-plan-request.json" \
  "$relay_url/api/v2/projects/codencer/run-plan" > "$TMPDIR_ROOT/run-plan-success.json"
grep -q '"ok":true\|"ok": true' "$TMPDIR_ROOT/run-plan-success.json" || { cat "$TMPDIR_ROOT/run-plan-success.json" >&2; exit 1; }

run_id="$(python3 - "$TMPDIR_ROOT/run-plan-success.json" <<'PY'
import json, sys
payload=json.load(open(sys.argv[1]))
print(payload.get("run", {}).get("id", ""))
PY
)"
test -n "$run_id"
curl -fsS -H "Authorization: Bearer $planner_token" \
  "$relay_url/api/v2/projects/codencer/reports/run-plans/$run_id" > "$TMPDIR_ROOT/report.json"
grep -q "$run_id" "$TMPDIR_ROOT/report.json" || { cat "$TMPDIR_ROOT/report.json" >&2; exit 1; }

python3 - "$ROOT/testdata/manifests/fake-blocker.yaml" > "$TMPDIR_ROOT/run-plan-blocker-request.json" <<'PY'
import json, sys
print(json.dumps({"manifest_text": open(sys.argv[1]).read(), "manifest_name": "fake-blocker.yaml", "wait": True}))
PY
curl -fsS \
  -H "Authorization: Bearer $planner_token" \
  -H "Content-Type: application/json" \
  -d @"$TMPDIR_ROOT/run-plan-blocker-request.json" \
  "$relay_url/api/v2/projects/codencer/run-plan" > "$TMPDIR_ROOT/run-plan-blocker.json"
grep -q '"exit_code":10\|"exit_code": 10' "$TMPDIR_ROOT/run-plan-blocker.json" || { cat "$TMPDIR_ROOT/run-plan-blocker.json" >&2; exit 1; }
grep -q '"type":"question"\|"type": "question"' "$TMPDIR_ROOT/run-plan-blocker.json" || { cat "$TMPDIR_ROOT/run-plan-blocker.json" >&2; exit 1; }

python3 - "$ROOT/testdata/manifests/fake-validation-failure.yaml" > "$TMPDIR_ROOT/run-plan-validation-request.json" <<'PY'
import json, sys
print(json.dumps({"manifest_text": open(sys.argv[1]).read(), "manifest_name": "fake-validation-failure.yaml", "wait": True}))
PY
curl -fsS \
  -H "Authorization: Bearer $planner_token" \
  -H "Content-Type: application/json" \
  -d @"$TMPDIR_ROOT/run-plan-validation-request.json" \
  "$relay_url/api/v2/projects/codencer/run-plan" > "$TMPDIR_ROOT/run-plan-validation.json"
grep -q '"exit_code":21\|"exit_code": 21' "$TMPDIR_ROOT/run-plan-validation.json" || { cat "$TMPDIR_ROOT/run-plan-validation.json" >&2; exit 1; }
grep -q '"type":"validation_failed"\|"type": "validation_failed"' "$TMPDIR_ROOT/run-plan-validation.json" || { cat "$TMPDIR_ROOT/run-plan-validation.json" >&2; exit 1; }

curl -fsS \
  -H "Authorization: Bearer $planner_token" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"name":"codencer.list_projects","arguments":{}}' \
  "$relay_url/mcp/call" > "$TMPDIR_ROOT/mcp-list-projects.json"
grep -q '"codencer"' "$TMPDIR_ROOT/mcp-list-projects.json" || { cat "$TMPDIR_ROOT/mcp-list-projects.json" >&2; exit 1; }
grep -q '"locations"' "$TMPDIR_ROOT/mcp-list-projects.json" || { cat "$TMPDIR_ROOT/mcp-list-projects.json" >&2; exit 1; }
if grep -q "$repo" "$TMPDIR_ROOT/mcp-list-projects.json"; then
  echo "MCP list_projects leaked absolute repo path" >&2
  cat "$TMPDIR_ROOT/mcp-list-projects.json" >&2
  exit 1
fi

kill "$CONNECTOR_PID" 2>/dev/null || true
wait "$CONNECTOR_PID" 2>/dev/null || true
CONNECTOR_PID=""
"$ROOT/bin/codencer-connectord" run --config "$connector_config" > "$TMPDIR_ROOT/connector-restarted.log" 2>&1 &
CONNECTOR_PID="$!"
for _ in $(seq 1 150); do
  curl -fsS -H "Authorization: Bearer $planner_token" "$relay_url/api/v2/projects" > "$projects_json" || true
  if grep -q '"project_id":"codencer"\|"project_id": "codencer"' "$projects_json"; then
    break
  fi
  sleep 0.1
done
grep -q '"project_id":"codencer"\|"project_id": "codencer"' "$projects_json" || { cat "$projects_json" >&2; exit 1; }

echo "local relay MCP verification passed"
