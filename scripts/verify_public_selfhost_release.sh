#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-public-selfhost.XXXXXX")"

cleanup() {
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

assert_no_commercial_endpoint() {
  local file="$1"
  if grep -q 'https://mcp.codencer.dev\|https://relay.codencer.dev\|https://app.codencer.dev' "$file"; then
    echo "commercial endpoint appeared in public self-host proof: $file" >&2
    cat "$file" >&2
    exit 1
  fi
}

home="$TMPDIR_ROOT/home"
gateway_port="$(free_port)"
relay_port="$(free_port)"
console_port="$(free_port)"
gateway_url="http://127.0.0.1:$gateway_port"
relay_url="http://127.0.0.1:$relay_port"
console_url="http://127.0.0.1:$console_port"

echo "Public self-host release verification ports: gateway=$gateway_port relay=$relay_port console=$console_port"

export CODENCER_HOME="$home"
"$ROOT/bin/codencer" init --json > "$TMPDIR_ROOT/init.json"
"$ROOT/bin/codencer" config show --json > "$TMPDIR_ROOT/config-default.json"
assert_no_commercial_endpoint "$TMPDIR_ROOT/config-default.json"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "resolved_connection.gateway_url")" = "http://127.0.0.1:19090"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "resolved_connection.mcp_url")" = "http://127.0.0.1:19090/mcp"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "resolved_connection.relay_url")" = "http://127.0.0.1:8090"
test "$(json_get "$TMPDIR_ROOT/config-default.json" "config.active_profile")" = "self-host"

"$ROOT/bin/codencer" config profiles list --json > "$TMPDIR_ROOT/profiles.json"
grep -q '"name": "self-host"' "$TMPDIR_ROOT/profiles.json" || { cat "$TMPDIR_ROOT/profiles.json" >&2; exit 1; }

CODENCER_GATEWAY_URL="$gateway_url" "$ROOT/bin/codencer" config show --json > "$TMPDIR_ROOT/config-env.json"
test "$(json_get "$TMPDIR_ROOT/config-env.json" "resolved_connection.gateway_url")" = "$gateway_url"
test "$(json_get "$TMPDIR_ROOT/config-env.json" "resolved_connection.source")" = "env:CODENCER_GATEWAY_URL"

"$ROOT/bin/codencer" config set gateway.url "$gateway_url" --json > "$TMPDIR_ROOT/config-set.json"
"$ROOT/bin/codencer" config set relay.url "$relay_url" --json > "$TMPDIR_ROOT/config-set-relay.json"
"$ROOT/bin/codencer" config set console.url "$console_url" --json > "$TMPDIR_ROOT/config-set-console.json"
"$ROOT/bin/codencer" config profiles use self-host --json > "$TMPDIR_ROOT/profile-use.json"
"$ROOT/bin/codencer" config show --json > "$TMPDIR_ROOT/config-profile.json"
test "$(json_get "$TMPDIR_ROOT/config-profile.json" "resolved_connection.gateway_url")" = "$gateway_url"
test "$(json_get "$TMPDIR_ROOT/config-profile.json" "resolved_connection.relay_url")" = "$relay_url"
test "$(json_get "$TMPDIR_ROOT/config-profile.json" "resolved_connection.console_url")" = "$console_url"

"$ROOT/bin/codencer" setup self-host \
  --gateway-url "$gateway_url" \
  --mcp-url "$gateway_url/mcp" \
  --relay-url "$relay_url" \
  --console-url "$console_url" \
  --listen "127.0.0.1:$gateway_port" \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --default-relay-token-env CODENCER_DEFAULT_RELAY_TOKEN \
  --enable-oauth-dev \
  --oauth-client-secret public-selfhost-client-secret \
  --json > "$TMPDIR_ROOT/setup-selfhost.json"
assert_no_commercial_endpoint "$TMPDIR_ROOT/setup-selfhost.json"
grep -q '"mode": "self-host"' "$TMPDIR_ROOT/setup-selfhost.json" || { cat "$TMPDIR_ROOT/setup-selfhost.json" >&2; exit 1; }
if grep -q 'public-selfhost-client-secret' "$TMPDIR_ROOT/setup-selfhost.json"; then
  echo "setup self-host leaked OAuth client secret" >&2
  cat "$TMPDIR_ROOT/setup-selfhost.json" >&2
  exit 1
fi

"$ROOT/bin/codencer" setup mcp --client codex --json > "$TMPDIR_ROOT/codex-mcp.json"
"$ROOT/bin/codencer" setup mcp --client claude-code --json > "$TMPDIR_ROOT/claude-mcp.json"
"$ROOT/bin/codencer" activation self-host \
  --gateway "$gateway_url" \
  --relay "$relay_url" \
  --project codencer \
  --token-env CODENCER_GATEWAY_MCP_TOKEN \
  --json > "$TMPDIR_ROOT/chatgpt-sheet.json"
chatgpt_package="$(json_get "$TMPDIR_ROOT/chatgpt-sheet.json" "package_path")"
chatgpt_sheet="$chatgpt_package/chatgpt-app-setup.md"
test -f "$chatgpt_sheet"
for file in "$TMPDIR_ROOT/codex-mcp.json" "$TMPDIR_ROOT/claude-mcp.json" "$TMPDIR_ROOT/chatgpt-sheet.json" "$chatgpt_sheet"; do
  assert_no_commercial_endpoint "$file"
  grep -q "$gateway_url/mcp" "$file" || { cat "$file" >&2; exit 1; }
done
grep -q 'codex' "$TMPDIR_ROOT/codex-mcp.json" || { cat "$TMPDIR_ROOT/codex-mcp.json" >&2; exit 1; }
grep -q 'claude mcp add --transport http' "$TMPDIR_ROOT/claude-mcp.json" || { cat "$TMPDIR_ROOT/claude-mcp.json" >&2; exit 1; }
grep -q 'ChatGPT' "$chatgpt_sheet" || { cat "$chatgpt_sheet" >&2; exit 1; }

gateway_config="$home/runtime/gateway/config.json"
test -f "$gateway_config"
assert_no_commercial_endpoint "$gateway_config"
if grep -q 'public-selfhost-client-secret\|gateway-token\|relay-token' "$gateway_config"; then
  echo "gateway config leaked token-like material" >&2
  cat "$gateway_config" >&2
  exit 1
fi

echo "--- Public Self-Host Release Config Proof ---"
echo "Gateway default: http://127.0.0.1:19090"
echo "Gateway override: $gateway_url"
echo "Relay override:   $relay_url"
echo "Console override: $console_url"
echo "Codex config:     $TMPDIR_ROOT/codex-mcp.json"
echo "Claude command:   $TMPDIR_ROOT/claude-mcp.json"
echo "ChatGPT sheet:    $chatgpt_sheet"
