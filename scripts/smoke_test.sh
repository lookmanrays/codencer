#!/usr/bin/env bash
# scripts/smoke_test.sh - Verify the local bridge relay loop.
set -e

# Default to simulation mode unless overridden
export ALL_ADAPTERS_SIMULATION_MODE=${ALL_ADAPTERS_SIMULATION_MODE:-1}
export HOST=${HOST:-127.0.0.1}
export PORT=${PORT:-8085}

echo "--- Codencer Smoke Test ---"
echo "Mode: $([ "$ALL_ADAPTERS_SIMULATION_MODE" == "1" ] && echo "Simulation" || echo "Real")"

# 1. Setup & Build
make setup build > /dev/null

# 2. Start Daemon
echo "Checking for daemon on $HOST:$PORT..."
if curl -s http://$HOST:$PORT/api/v1/compatibility | grep -q '"tier"'; then
    echo "Daemon already running on $HOST:$PORT. Using existing daemon."
    DAEMON_ALREADY_RUNNING=1
else
    echo "Starting daemon on $HOST:$PORT..."
    ./bin/orchestratord > .codencer/smoke_daemon.log 2>&1 &
    DAEMON_PID=$!
    DAEMON_ALREADY_RUNNING=0
fi

function cleanup {
    if [ "$DAEMON_ALREADY_RUNNING" == "0" ]; then
        echo "Stopping daemon (PID: $DAEMON_PID)..."
        kill $DAEMON_PID || true
    fi
}
trap cleanup EXIT

# Wait for health check
echo "Waiting for daemon to be ready..."
for i in {1..10}; do
    if curl -s http://$HOST:$PORT/api/v1/compatibility | grep -q '"tier"'; then
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

echo "1. Starting run mission: $RUN_ID"
./bin/orchestratorctl run start "$RUN_ID" "$PROJECT" > /dev/null

echo "2. Submitting task (Flow: submit --wait)..."
cat <<EOF > .codencer/smoke_task.yaml
version: "1.1"
run_id: "$RUN_ID"
title: "Smoke Test Validation"
goal: "Verify the bridge relay loop"
adapter_profile: "codex"
is_simulation: true
EOF

# Execute and capture JSON output
./bin/orchestratorctl submit "$RUN_ID" .codencer/smoke_task.yaml --wait > .codencer/smoke_result.json

# Extract State and ID robustly without jq
# Initial Step JSON is first, final ResultSpec JSON is last.
STATE=$(grep -o '"state":[[:space:]]*"[^"]*"' .codencer/smoke_result.json | tail -1 | cut -d'"' -f4)
STEP_ID=$(grep -o '"id":[[:space:]]*"[^"]*"' .codencer/smoke_result.json | head -1 | cut -d'"' -f4)

echo "--- SMOKE TEST SUMMARY ---"
echo "UUID Handle:    $STEP_ID"
echo "Terminal State: $STATE"

if [[ "$STATE" == "completed" || "$STATE" == "completed_with_warnings" || "$STATE" == "failed_terminal" ]]; then
    echo "SUCCESS: Bridge relay loop verified."
    echo ""
    echo "Authoritative Result Summary:"
    ./bin/orchestratorctl step result "$STEP_ID"
else
    echo "FAILURE: Unexpected state reached."
    cat .codencer/smoke_result.json
    exit 1
fi
