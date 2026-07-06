#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/codencer-local-exec.XXXXXX")"
DAEMON_PID=""

cleanup() {
  if [[ -n "$DAEMON_PID" ]] && kill -0 "$DAEMON_PID" 2>/dev/null; then
    kill "$DAEMON_PID" 2>/dev/null || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
  rm -rf "$TMPDIR_ROOT"
}
trap cleanup EXIT

repo="$TMPDIR_ROOT/repo"
home="$TMPDIR_ROOT/home"
state="$TMPDIR_ROOT/state"
mkdir -p "$repo" "$home" "$state"

git -C "$repo" init -q
printf 'local execution verification\n' > "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" -c user.name=Codencer -c user.email=codencer@example.invalid commit -q -m "initial"

port="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"

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

daemon_url="http://127.0.0.1:$port"
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
if ! curl -fsS "$daemon_url/health" >/dev/null; then
  echo "daemon did not become healthy at $daemon_url; log follows" >&2
  cat "$TMPDIR_ROOT/daemon.log" >&2 || true
  exit 1
fi

export CODENCER_HOME="$home"
"$ROOT/bin/codencer" init --json >/dev/null
"$ROOT/bin/codencer" project init --id codencer --repo "$repo" --adapter fake --profile fake-success --daemon-url "$daemon_url" --json >/dev/null

"$ROOT/bin/codencer" submit --project codencer --goal "fake success submit" --profile fake-success --wait --json > "$TMPDIR_ROOT/submit-success.json"
"$ROOT/bin/codencer" run-plan "$ROOT/testdata/manifests/fake-success.yaml" --project codencer --wait --json > "$TMPDIR_ROOT/run-plan-success.json"

set +e
"$ROOT/bin/codencer" run-plan "$ROOT/testdata/manifests/fake-blocker.yaml" --project codencer --wait --json > "$TMPDIR_ROOT/run-plan-blocker.json"
blocker_code=$?
"$ROOT/bin/codencer" run-plan "$ROOT/testdata/manifests/fake-validation-failure.yaml" --project codencer --wait --json > "$TMPDIR_ROOT/run-plan-validation.json"
validation_code=$?
set -e

if [[ "$blocker_code" -ne 10 ]]; then
  echo "expected fake blocker run-plan exit 10, got $blocker_code" >&2
  cat "$TMPDIR_ROOT/run-plan-blocker.json" >&2
  exit 1
fi
if [[ "$validation_code" -ne 21 ]]; then
  echo "expected validation-failure run-plan exit 21, got $validation_code" >&2
  cat "$TMPDIR_ROOT/run-plan-validation.json" >&2
  exit 1
fi

grep -q '"ok": true' "$TMPDIR_ROOT/submit-success.json" || { cat "$TMPDIR_ROOT/submit-success.json" >&2; exit 1; }
grep -q '"ok": true' "$TMPDIR_ROOT/run-plan-success.json" || { cat "$TMPDIR_ROOT/run-plan-success.json" >&2; exit 1; }
grep -q '"type": "question"' "$TMPDIR_ROOT/run-plan-blocker.json" || { cat "$TMPDIR_ROOT/run-plan-blocker.json" >&2; exit 1; }
grep -q '"type": "validation_failed"' "$TMPDIR_ROOT/run-plan-validation.json" || { cat "$TMPDIR_ROOT/run-plan-validation.json" >&2; exit 1; }

echo "local execution verification passed"
