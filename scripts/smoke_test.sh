#!/usr/bin/env bash
# scripts/smoke_test.sh - Verify the local bridge relay loop.
set -e

# Default to simulation mode unless overridden
export ALL_ADAPTERS_SIMULATION_MODE=${ALL_ADAPTERS_SIMULATION_MODE:-1}
export SMOKE_PORT=${SMOKE_PORT:-8085}

echo "--- Codencer Smoke Test ---"
echo "Mode: $([ "$ALL_ADAPTERS_SIMULATION_MODE" == "1" ] && echo "Simulation" || echo "Real")"

# 1. Setup & Build
make setup build > /dev/null

# 2. Start Daemon
echo "Starting daemon on port $SMOKE_PORT..."
# Ensure we use a clean temp DB if needed, but for smoke test we just use the default .codencer/
# (Actually, let's use the default to verify the dev flow)
# We override the port via a future config override or just by letting it be for now.
# Since we don't have ENV override for PORT yet, we'll just use the default 8080 or assume it's free.
./bin/orchestratord > .codencer/smoke_daemon.log 2>&1 &
DAEMON_PID=$!

function cleanup {
    echo "Stopping daemon (PID: $DAEMON_PID)..."
    kill $DAEMON_PID || true
}
trap cleanup EXIT

# Wait for health check
echo "Waiting for daemon to be ready..."
for i in {1..10}; do
    if curl -s http://127.0.0.1:8080/health | grep -q "ok"; then
        echo "Daemon ready."
        break
    fi
    if [ $i -eq 10 ]; then
        echo "ERROR: Daemon failed to start. Check .codencer/smoke_daemon.log"
        exit 1
    fi
    sleep 1
done

# 3. Execute Relay Loop
RUN_ID="smoke-run-$(date +%s)"
PROJECT="smoke-project"

echo "1. Starting run: $RUN_ID"
./bin/orchestratorctl run start "$RUN_ID" "$PROJECT" --force > /dev/null

echo "2. Submitting simulation task..."
# Use a simple inline YAML for the smoke test
cat <<EOF > .codencer/smoke_task.yaml
title: "Smoke Test Task"
goal: "Verify the bridge relay loop"
adapter: "codex"
is_simulation: true
EOF

STEP_OUTPUT=$(./bin/orchestratorctl submit "$RUN_ID" .codencer/smoke_task.yaml)
# Extract step_id from JSON output
STEP_ID=$(echo "$STEP_OUTPUT" | grep -oP '"step_id":"\K[^"]+')

echo "3. Waiting for terminal state (Step: $STEP_ID)..."
./bin/orchestratorctl step wait "$STEP_ID" --timeout 30s > .codencer/smoke_result.json

echo "--- RESULTS ---"
# Check if result is valid JSON and has a terminal state
STATE=$(grep -oP '"state":"\K[^"]+' .codencer/smoke_result.json)
echo "Terminal State: $STATE"

if [[ "$STATE" == "completed" || "$STATE" == "completed_with_warnings" ]]; then
    echo "SUCCESS: Smoke test passed!"
else
    echo "FAILURE: Unexpected terminal state: $STATE"
    cat .codencer/smoke_result.json
    exit 1
fi
