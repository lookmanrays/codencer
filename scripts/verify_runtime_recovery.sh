#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-runtime-recovery.XXXXXX")"
DAEMON_PID=""

cleanup() {
  if [[ -n "$DAEMON_PID" ]] && kill -0 "$DAEMON_PID" 2>/dev/null; then
    kill "$DAEMON_PID" 2>/dev/null || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
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

repo="$TMPDIR_ROOT/repo"
home="$TMPDIR_ROOT/home"
state="$TMPDIR_ROOT/state"
mkdir -p "$repo" "$home" "$state"

git -C "$repo" init -q
printf 'runtime recovery verification\n' > "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" -c user.name=Codencer -c user.email=codencer@example.invalid commit -q -m "initial"

port="$(free_port)"
daemon_url="http://127.0.0.1:$port"
daemon_config="$TMPDIR_ROOT/daemon.json"
cat > "$daemon_config" <<JSON
{
  "log_level": "error",
  "db_path": "$state/codencer.db",
  "artifact_root": "$state/artifacts",
  "workspace_root": "$state/workspace",
  "repo_root": "$repo",
  "host": "127.0.0.1",
  "port": $port
}
JSON

export CODENCER_HOME="$home"
"$ROOT/bin/codencer" init --json >/dev/null
"$ROOT/bin/codencer" project init --id codencer --repo "$repo" --adapter fake --profile fake-success --daemon-url "$daemon_url" --json >/dev/null

"$ROOT/bin/codencer" service status --all --json --manager manual --bin-dir "$ROOT/bin" > "$TMPDIR_ROOT/service-status.json"
"$ROOT/bin/codencer" service install --all --dry-run --json --manager launchd --bin-dir "$ROOT/bin" > "$TMPDIR_ROOT/install-launchd.json"
"$ROOT/bin/codencer" service install --all --dry-run --json --manager systemd --bin-dir "$ROOT/bin" > "$TMPDIR_ROOT/install-systemd.json"
"$ROOT/bin/codencer" service render daemon --format launchd --bin-dir "$ROOT/bin" > "$TMPDIR_ROOT/daemon.plist"
"$ROOT/bin/codencer" service render daemon --format systemd --bin-dir "$ROOT/bin" > "$TMPDIR_ROOT/daemon.service"
"$ROOT/bin/codencer" service logs daemon --tail 100 --manager manual --bin-dir "$ROOT/bin" > "$TMPDIR_ROOT/logs.txt"
"$ROOT/bin/codencer" watchdog once --json --manager manual --bin-dir "$ROOT/bin" > "$TMPDIR_ROOT/watchdog-down.json"
"$ROOT/bin/codencer" recover --dry-run --json --manager manual --bin-dir "$ROOT/bin" > "$TMPDIR_ROOT/recover-dry-run.json"
"$ROOT/bin/codencer" recover locks --dry-run --json --manager manual --bin-dir "$ROOT/bin" > "$TMPDIR_ROOT/recover-locks.json"

grep -q 'io.codencer.daemon' "$TMPDIR_ROOT/daemon.plist"
grep -q 'ExecStart=' "$TMPDIR_ROOT/daemon.service"
grep -q '"daemon_not_running"' "$TMPDIR_ROOT/watchdog-down.json"
if grep -Eiq 'bearer [A-Za-z0-9]|private_key|planner_token' "$TMPDIR_ROOT"/*.json; then
  echo "unexpected secret-like value in runtime recovery output" >&2
  exit 1
fi

env \
  PORT="$port" \
  HOST="127.0.0.1" \
  DB_PATH="$state/codencer.db" \
  ARTIFACT_ROOT="$state/artifacts" \
  WORKSPACE_ROOT="$state/workspace" \
  LOG_LEVEL="error" \
  REPO_ROOT="$repo" \
  "$ROOT/bin/orchestratord" --config "$daemon_config" --repo-root "$repo" > "$TMPDIR_ROOT/daemon.log" 2>&1 &
DAEMON_PID="$!"

for _ in $(seq 1 150); do
  if curl -fsS "$daemon_url/health" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$DAEMON_PID" 2>/dev/null; then
    echo "daemon exited early; log follows" >&2
    cat "$TMPDIR_ROOT/daemon.log" >&2 || true
    exit 1
  fi
  sleep 0.1
done
curl -fsS "$daemon_url/health" >/dev/null

"$ROOT/bin/codencer" run-plan "$ROOT/testdata/manifests/fake-success.yaml" --project codencer --wait --json > "$TMPDIR_ROOT/run-plan-success.json"
grep -q '"ok": true' "$TMPDIR_ROOT/run-plan-success.json" || { cat "$TMPDIR_ROOT/run-plan-success.json" >&2; exit 1; }

echo "runtime recovery verification passed"
