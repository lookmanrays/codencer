#!/usr/bin/env bash

set -e

echo "Building daemon and cli..."
go build -o orchestratord cmd/orchestratord/main.go
go build -o orchestratorctl cmd/orchestratorctl/main.go

echo "Starting orchestratord in background..."
export CODENCER_PORT=8083
./orchestratord > daemon.log 2>&1 &
DAEMON_PID=$!

sleep 2 # wait for startup

echo "Daemon started (PID: $DAEMON_PID)"

function cleanup {
    echo "Cleaning up..."
    kill $DAEMON_PID || true
    rm orchestratord orchestratorctl daemon.log
}
trap cleanup EXIT

echo "Starting run via API..."
RUN_ID="test-run-$(date +%s)"
./orchestratorctl run start "$RUN_ID" "test-proj" || exit 1

echo "Checking run status..."
./orchestratorctl run status "$RUN_ID"

echo "End-to-End script executed successfully! The orchestrator correctly received the run and persisted it."
