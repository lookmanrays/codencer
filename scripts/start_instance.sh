#!/bin/bash
set -e

# Codencer Instance Startup Helper
# Usage: ./scripts/start_instance.sh <repo_root> <port> [extra_flags]

REPO_ROOT=$1
PORT=${2:-8085}
EXTRA_FLAGS=${@:3}

if [ -z "$REPO_ROOT" ]; then
    echo "Usage: $0 <repo_root> [port] [extra_flags]"
    echo "Example: $0 ~/projects/my-repo 8086"
    exit 1
fi

# Resolve absolute path for REPO_ROOT
ABS_REPO_ROOT=$(cd "$REPO_ROOT" && pwd)

echo "==> Starting Codencer for: $ABS_REPO_ROOT"
echo "==> Port: $PORT"

# Ensure binaries are built
if [ ! -f "./bin/orchestratord" ]; then
    echo "==> Binaries missing. Running 'make build'..."
    make build
fi

# Start the daemon
PORT=$PORT ./bin/orchestratord --repo-root "$ABS_REPO_ROOT" $EXTRA_FLAGS
