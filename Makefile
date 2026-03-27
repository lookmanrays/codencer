-include .env
export

all: lint test build

build:
	@echo "==> Building orchestratord..."
	@go build -ldflags "-X agent-bridge/internal/app.Version=v0.1.0" -o bin/orchestratord ./cmd/orchestratord
	@echo "==> Building orchestratorctl..."
	@go build -ldflags "-X agent-bridge/internal/app.Version=v0.1.0" -o bin/orchestratorctl ./cmd/orchestratorctl

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
	@nohup ./bin/orchestratord > .codencer/daemon.log 2>&1 & echo $$! > .codencer/daemon.pid
	@echo "Daemon started (PID: $$(cat .codencer/daemon.pid)). Logs: .codencer/daemon.log"

stop:
	@echo "==> Stopping orchestratord..."
	@kill $$(cat .codencer/daemon.pid) && rm .codencer/daemon.pid || echo "No daemon running."

start-sim: build setup
	@echo "==> Starting orchestratord in SIMULATION MODE (background)..."
	@nohup ALL_ADAPTERS_SIMULATION_MODE=1 ./bin/orchestratord > .codencer/daemon.log 2>&1 & echo $$! > .codencer/daemon.pid
	@echo "Simulated daemon started (PID: $$(cat .codencer/daemon.pid)). Logs: .codencer/daemon.log"

setup:
	@echo "==> Initializing local environment (.codencer/)..."
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
