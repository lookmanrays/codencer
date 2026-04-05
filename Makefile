all: lint test build

build:
	@echo "==> Building orchestratord..."
	@go build -ldflags "-X agent-bridge/internal/app.Version=v0.1.0-beta" -o bin/orchestratord ./cmd/orchestratord
	@echo "==> Building orchestratorctl..."
	@go build -ldflags "-X agent-bridge/internal/app.Version=v0.1.0-beta" -o bin/orchestratorctl ./cmd/orchestratorctl

test:
	@echo "==> Running tests..."
	@go test -v ./...

lint:
	@echo "==> Linting code..."
	@golangci-lint run ./... || echo "golangci-lint not installed or failed"

run: build
	@echo "==> Running orchestratord..."
	@./bin/orchestratord

dev: setup build
	@echo "==> Starting local dev daemon..."
	@./bin/orchestratord

start: build setup
	@echo "==> Starting orchestratord in background..."
	@if [ -f .env ]; then source .env; fi; \
	PORT=$${PORT:-8085}; \
	if curl -s http://127.0.0.1:$$PORT/health | grep -q "ok"; then \
		echo "Daemon already running and healthy on port $$PORT."; \
		exit 0; \
	fi; \
	nohup ./bin/orchestratord > .codencer/daemon.log 2>&1 & echo $$! > .codencer/daemon.pid; \
	echo "Waiting for health check..."; \
	for i in $$(seq 1 10); do \
		if curl -s http://127.0.0.1:$$PORT/health | grep -q "ok"; then \
			echo "Daemon successfully started (PID: $$(cat .codencer/daemon.pid)). Logs: .codencer/daemon.log"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "ERROR: Daemon failed to start. Check .codencer/daemon.log"; \
	kill $$(cat .codencer/daemon.pid) 2>/dev/null || true; \
	exit 1

stop:
	@echo "==> Stopping orchestratord..."
	@if [ -f .codencer/daemon.pid ]; then \
		pid=$$(cat .codencer/daemon.pid); \
		if kill -0 $$pid 2>/dev/null; then \
			kill $$pid; \
			echo "Daemon stopped."; \
		else \
			echo "Daemon not running (stale pid)."; \
		fi; \
		rm -f .codencer/daemon.pid; \
	else \
		echo "No daemon running (no pid file)."; \
	fi

start-sim: build setup
	@echo "==> Starting orchestratord in SIMULATION MODE (background)..."
	@if [ -f .env ]; then source .env; fi; \
	PORT=$${PORT:-8085}; \
	if curl -s http://127.0.0.1:$$PORT/health | grep -q "ok"; then \
		echo "Daemon already running and healthy on port $$PORT."; \
		exit 0; \
	fi; \
	nohup env ALL_ADAPTERS_SIMULATION_MODE=1 ./bin/orchestratord > .codencer/daemon.log 2>&1 & echo $$! > .codencer/daemon.pid; \
	echo "Waiting for health check..."; \
	for i in $$(seq 1 10); do \
		if curl -s http://127.0.0.1:$$PORT/health | grep -q "ok"; then \
			echo "Simulated daemon successfully started (PID: $$(cat .codencer/daemon.pid)). Logs: .codencer/daemon.log"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "ERROR: Simulated daemon failed to start. Check .codencer/daemon.log"; \
	kill $$(cat .codencer/daemon.pid) 2>/dev/null || true; \
	exit 1

setup:
	@echo "==> Initializing local environment (.codencer/)..."
	@if [ ! -f .env ]; then \
		echo "==> Creating .env from .env.example..."; \
		cp .env.example .env; \
	fi
	@mkdir -p bin
	@mkdir -p .codencer/artifacts
	@mkdir -p .codencer/workspace

doctor: build
	@echo "==> Verifying local environment using orchestratorctl..."
	@./bin/orchestratorctl doctor

clean:
	@echo "==> Cleaning up build artifacts..."
	@rm -rf bin
	@echo "Note: Use 'make nuke' to delete the database and local history."

nuke: clean
	@echo "==> NUKING local database and workspace..."
	@rm -rf .codencer

simulate: build
	@echo "==> Running in ALL-ADAPTERS SIMULATION MODE..."
	@ALL_ADAPTERS_SIMULATION_MODE=1 ./bin/orchestratord

smoke: build
	@echo "==> Running automated smoke test..."
	@./scripts/smoke_test.sh

validate: build
	@echo "==> Running Codex validation scenario (Internal Version Bump)..."
	@./bin/orchestratorctl run start validation-run-01 validation-project || true
	@./bin/orchestratorctl submit validation-run-01 docs/validation_task.yaml
	@./bin/orchestratorctl step wait bump-version-01
