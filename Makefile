VERSION ?= v0.2.0-beta
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DIRTY ?= $(shell test -z "$$(git status --porcelain 2>/dev/null)" && echo false || echo true)
LDFLAGS := -X agent-bridge/internal/app.Version=$(VERSION) -X agent-bridge/internal/buildinfo.Version=$(VERSION) -X agent-bridge/internal/buildinfo.Commit=$(COMMIT) -X agent-bridge/internal/buildinfo.Date=$(BUILD_DATE) -X agent-bridge/internal/buildinfo.Dirty=$(DIRTY)

all: lint test build

build-supported: build build-cloud build-mcp-sdk-smoke

build: build-codencer
	@mkdir -p bin
	@echo "==> Building orchestratord..."
	@go build -ldflags "$(LDFLAGS)" -o bin/orchestratord ./cmd/orchestratord
	@echo "==> Building orchestratorctl..."
	@go build -ldflags "$(LDFLAGS)" -o bin/orchestratorctl ./cmd/orchestratorctl
	@echo "==> Building codencer-connectord..."
	@go build -ldflags "$(LDFLAGS)" -o bin/codencer-connectord ./cmd/codencer-connectord
	@echo "==> Building codencer-relayd..."
	@go build -ldflags "$(LDFLAGS)" -o bin/codencer-relayd ./cmd/codencer-relayd

build-codencer:
	@mkdir -p bin
	@echo "==> Building codencer..."
	@go build -ldflags "$(LDFLAGS)" -o bin/codencer ./cmd/codencer

build-orchestratord:
	@mkdir -p bin
	@echo "==> Building orchestratord..."
	@go build -ldflags "$(LDFLAGS)" -o bin/orchestratord ./cmd/orchestratord

build-cloud:
	@mkdir -p bin
	@echo "==> Building codencer-cloudctl..."
	@go build -ldflags "$(LDFLAGS)" -o bin/codencer-cloudctl ./cmd/codencer-cloudctl
	@echo "==> Building codencer-cloudd..."
	@go build -ldflags "$(LDFLAGS)" -o bin/codencer-cloudd ./cmd/codencer-cloudd
	@echo "==> Building codencer-cloudworkerd..."
	@go build -ldflags "$(LDFLAGS)" -o bin/codencer-cloudworkerd ./cmd/codencer-cloudworkerd

build-broker:
	@mkdir -p bin
	@echo "==> Building agent-broker (nested module)..."
	@cd cmd/broker && go build -o ../../bin/agent-broker ./...

build-mcp-sdk-smoke:
	@mkdir -p bin
	@echo "==> Building mcp-sdk-smoke (official MCP SDK proof helper)..."
	@go build -o bin/mcp-sdk-smoke ./cmd/mcp-sdk-smoke

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
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	HOST=$${HOST:-127.0.0.1}; \
	PORT=$${PORT:-8085}; \
	if curl -s http://$$HOST:$$PORT/api/v1/compatibility | grep -q '"tier"'; then \
		echo "Daemon already running and healthy on $$HOST:$$PORT."; \
		exit 0; \
	fi; \
	REPO_ROOT_VAL=$${REPO_ROOT}; \
	if [ -n "$$REPO_ROOT_VAL" ]; then REPO_ROOT_FLAG="--repo-root $$REPO_ROOT_VAL"; fi; \
	nohup ./bin/orchestratord $$REPO_ROOT_FLAG > .codencer/daemon.log 2>&1 & echo $$! > .codencer/daemon.pid; \
	echo "Waiting for health check..."; \
	for i in $$(seq 1 10); do \
		if curl -s http://$$HOST:$$PORT/api/v1/compatibility | grep -q '"tier"'; then \
			echo "Daemon successfully started on http://$$HOST:$$PORT (PID: $$(cat .codencer/daemon.pid)). Logs: .codencer/daemon.log"; \
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
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	HOST=$${HOST:-127.0.0.1}; \
	PORT=$${PORT:-8085}; \
	if curl -s http://$$HOST:$$PORT/api/v1/compatibility | grep -q '"tier"'; then \
		echo "Daemon already running and healthy on $$HOST:$$PORT."; \
		exit 0; \
	fi; \
	REPO_ROOT_VAL=$${REPO_ROOT}; \
	if [ -n "$$REPO_ROOT_VAL" ]; then REPO_ROOT_FLAG="--repo-root $$REPO_ROOT_VAL"; fi; \
	nohup env ALL_ADAPTERS_SIMULATION_MODE=1 ./bin/orchestratord $$REPO_ROOT_FLAG > .codencer/daemon.log 2>&1 & echo $$! > .codencer/daemon.pid; \
	echo "Waiting for health check..."; \
	for i in $$(seq 1 10); do \
		if curl -s http://$$HOST:$$PORT/api/v1/compatibility | grep -q '"tier"'; then \
			echo "Simulated daemon successfully started on http://$$HOST:$$PORT (PID: $$(cat .codencer/daemon.pid)). Logs: .codencer/daemon.log"; \
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

doctor-toolchain: build-codencer
	@echo "==> Verifying local production toolchain using codencer..."
	@./bin/codencer doctor toolchain --json

verify-local-prod: build-codencer
	@echo "==> Checking local production formatting..."
	@fmt=$$(gofmt -l internal/project internal/local cmd/codencer); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for local production files."; \
		exit 1; \
	fi
	@echo "==> Running local production unit tests..."
	@go test ./internal/project ./internal/local ./cmd/codencer
	@echo "==> Running local production CLI smoke..."
	@tmpdir=$$(mktemp -d "$${TMPDIR:-/tmp}/codencer-local-prod.XXXXXX"); \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer paths --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer init --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer doctor --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer doctor toolchain --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer project init --id codencer --repo . --adapter codex --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer project list --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer project status codencer --json >/dev/null
	@$(MAKE) verify-local-execution
	@$(MAKE) verify-local-relay-mcp
	@$(MAKE) verify-runtime-recovery
	@$(MAKE) verify-live-matrix
	@$(MAKE) acceptance-local-production

verify-local-execution: build-codencer build-orchestratord
	@echo "==> Checking local execution formatting..."
	@fmt=$$(gofmt -l internal/profile internal/manifest internal/localexec internal/adapters/fake internal/validation cmd/codencer); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for local execution files."; \
		exit 1; \
	fi
	@echo "==> Running local execution unit tests..."
	@go test ./internal/profile ./internal/manifest ./internal/localexec ./internal/adapters/fake ./internal/validation ./cmd/codencer
	@echo "==> Running local execution daemon smoke..."
	@./scripts/verify_local_execution.sh

verify-local-relay-mcp: build build-mcp-sdk-smoke
	@echo "==> Checking local relay/MCP formatting..."
	@fmt=$$(gofmt -l internal/connector internal/relay internal/relayproto cmd/codencer cmd/codencer-connectord cmd/codencer-relayd); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for local relay/MCP files."; \
		exit 1; \
	fi
	@echo "==> Running local relay/MCP unit tests..."
	@go test ./internal/project ./internal/connector ./internal/relay ./cmd/codencer ./cmd/codencer-connectord ./cmd/codencer-relayd
	@echo "==> Running local relay/MCP smoke..."
	@./scripts/verify_local_relay_mcp.sh

verify-runtime-recovery: build
	@echo "==> Checking runtime supervisor formatting..."
	@fmt=$$(gofmt -l internal/supervisor internal/local/config.go internal/service/recovery_service.go internal/app/bootstrap.go internal/app/routes.go cmd/codencer); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for runtime supervisor files."; \
		exit 1; \
	fi
	@echo "==> Running runtime supervisor unit tests..."
	@go test ./internal/supervisor ./internal/local ./internal/service ./internal/app ./cmd/codencer
	@echo "==> Running runtime supervisor smoke..."
	@bash ./scripts/verify_runtime_recovery.sh

verify-live-matrix: build-codencer
	@echo "==> Checking live matrix formatting..."
	@fmt=$$(gofmt -l internal/live internal/readiness internal/mcpconfig cmd/codencer cmd/codencer-relayd internal/relay/mcp_tools.go); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for live matrix files."; \
		exit 1; \
	fi
	@echo "==> Running live matrix unit tests..."
	@go test ./internal/live ./internal/readiness ./internal/mcpconfig ./cmd/codencer ./cmd/codencer-relayd ./internal/relay
	@echo "==> Running non-live live matrix smoke..."
	@tmpdir=$$(mktemp -d "$${TMPDIR:-/tmp}/codencer-live-matrix.XXXXXX"); \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer init --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer live matrix --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer live codex --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer live claude --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer live codex-mcp --endpoint https://relay.example.com/mcp --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer live claude-mcp --endpoint https://relay.example.com/mcp --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer live wsl --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer readiness --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer live reports --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer readiness reports --json >/dev/null

acceptance-local-production: build
	@echo "==> Running local production acceptance..."
	@tmpdir=$$(mktemp -d "$${TMPDIR:-/tmp}/codencer-acceptance.XXXXXX"); \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer init --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer accept local-production --json --bin-dir ./bin --repo . >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer accept reports --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer proof bundle --json --repo . >/dev/null

demo-local: build
	@echo "==> Running deterministic local demo..."
	@tmpdir=$$(mktemp -d "$${TMPDIR:-/tmp}/codencer-demo-home.XXXXXX"); \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer demo local --json --bin-dir ./bin >/dev/null

release-snapshot:
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required"; exit 2; fi
	@echo "==> Creating release snapshot $(VERSION)..."
	@go run ./internal/release/cmd --version "$(VERSION)" --dist dist --json

verify-release:
	@echo "==> Checking Sprint 6 formatting..."
	@fmt=$$(gofmt -l internal/buildinfo internal/security internal/setup internal/acceptance internal/proof internal/release cmd/codencer internal/relay/router.go); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for release hardening files."; \
		exit 1; \
	fi
	@echo "==> Running full Go test suite..."
	@go test ./...
	@$(MAKE) build
	@$(MAKE) verify-local-execution
	@$(MAKE) verify-local-relay-mcp
	@$(MAKE) verify-runtime-recovery
	@$(MAKE) verify-live-matrix
	@$(MAKE) acceptance-local-production
	@$(MAKE) demo-local
	@./scripts/install.sh --dry-run --json >/dev/null
	@./scripts/uninstall.sh --dry-run --json >/dev/null
	@./scripts/upgrade.sh --dry-run --json >/dev/null
	@$(MAKE) release-snapshot VERSION=v0.3.0-local-prod-verify

live-service-macos-smoke: build
	@if [ "$${CODENCER_LIVE_SERVICE_SMOKE:-0}" != "1" ]; then \
		echo "SKIP: set CODENCER_LIVE_SERVICE_SMOKE=1 to run live macOS service smoke."; \
		exit 0; \
	fi
	@./bin/codencer service install --all --manager launchd --json
	@./bin/codencer service start --all --manager launchd --json
	@./bin/codencer service status --all --manager launchd --json
	@./bin/codencer watchdog once --json
	@./bin/codencer service stop --all --manager launchd --json
	@./bin/codencer service uninstall --all --manager launchd --json

live-service-linux-smoke: build
	@if [ "$${CODENCER_LIVE_SERVICE_SMOKE:-0}" != "1" ]; then \
		echo "SKIP: set CODENCER_LIVE_SERVICE_SMOKE=1 to run live Linux service smoke."; \
		exit 0; \
	fi
	@./bin/codencer service install --all --manager systemd --json
	@./bin/codencer service start --all --manager systemd --json
	@./bin/codencer service status --all --manager systemd --json
	@./bin/codencer watchdog once --json
	@./bin/codencer service stop --all --manager systemd --json
	@./bin/codencer service uninstall --all --manager systemd --json

live-service-wsl-smoke: live-service-linux-smoke

live-codex-smoke: build-codencer build-orchestratord
	@if [ "$${CODENCER_LIVE_CODEX:-$${CODENCER_LIVE_CODEX_SMOKE:-0}}" != "1" ]; then \
		echo "SKIP: set CODENCER_LIVE_CODEX=1 to run live Codex smoke."; \
		exit 0; \
	fi
	@CODENCER_LIVE_CODEX=1 ./bin/codencer live codex --json --bin-dir ./bin --repo .

live-claude-smoke: build-codencer build-orchestratord
	@if [ "$${CODENCER_LIVE_CLAUDE:-$${CODENCER_LIVE_CLAUDE_SMOKE:-0}}" != "1" ]; then \
		echo "SKIP: set CODENCER_LIVE_CLAUDE=1 to run live Claude smoke."; \
		exit 0; \
	fi
	@CODENCER_LIVE_CLAUDE=1 ./bin/codencer live claude --json --bin-dir ./bin --repo .

live-relay-mcp-smoke: build
	@if [ "$${CODENCER_LIVE_RELAY_MCP:-0}" != "1" ]; then \
		echo "SKIP: set CODENCER_LIVE_RELAY_MCP=1 to run live Relay/MCP smoke."; \
		exit 0; \
	fi
	@CODENCER_LIVE_RELAY_MCP=1 ./bin/codencer live relay-mcp --json --bin-dir ./bin --repo .

live-codex-mcp-smoke: build-codencer
	@if [ "$${CODENCER_LIVE_CODEX_MCP:-0}" != "1" ]; then \
		echo "SKIP: set CODENCER_LIVE_CODEX_MCP=1 to run live Codex MCP client proof."; \
		exit 0; \
	fi
	@./bin/codencer live codex-mcp --json --endpoint "$${CODENCER_LIVE_MCP_ENDPOINT:-https://relay.example.com/mcp}"

live-claude-mcp-smoke: build-codencer
	@if [ "$${CODENCER_LIVE_CLAUDE_MCP:-0}" != "1" ]; then \
		echo "SKIP: set CODENCER_LIVE_CLAUDE_MCP=1 to run live Claude MCP client proof."; \
		exit 0; \
	fi
	@./bin/codencer live claude-mcp --json --endpoint "$${CODENCER_LIVE_MCP_ENDPOINT:-https://relay.example.com/mcp}"

live-wsl-smoke: build
	@if [ "$${CODENCER_LIVE_WSL:-0}" != "1" ]; then \
		echo "SKIP: set CODENCER_LIVE_WSL=1 to run live WSL smoke."; \
		exit 0; \
	fi
	@CODENCER_LIVE_WSL=1 ./bin/codencer live wsl --json --bin-dir ./bin --repo .

live-restart-reconnect-smoke: build
	@if [ "$${CODENCER_LIVE_SERVICE_RESTART:-0}" != "1" ]; then \
		echo "SKIP: set CODENCER_LIVE_SERVICE_RESTART=1 to run live restart/reconnect smoke."; \
		exit 0; \
	fi
	@CODENCER_LIVE_SERVICE_RESTART=1 ./bin/codencer live restart-reconnect --json --bin-dir ./bin --repo .

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

self-host-smoke: build
	@echo "==> Running self-host relay/connector smoke test..."
	@./scripts/self_host_smoke.sh

self-host-smoke-all: build build-mcp-sdk-smoke
	@echo "==> Running self-host relay/connector smoke test with all optional scenarios..."
	@SMOKE_SCENARIOS=all ./scripts/self_host_smoke.sh

self-host-smoke-mcp: build build-mcp-sdk-smoke
	@echo "==> Running self-host relay/connector smoke test with MCP coverage..."
	@SMOKE_SCENARIOS=status,audit,mcp,mcp-sdk ./scripts/self_host_smoke.sh

flagship-planner-smoke: build build-mcp-sdk-smoke
	@echo "==> Running flagship external-planner-to-local-Codex loop smoke..."
	@./scripts/flagship_planner_loop_smoke.sh

cloud-smoke: build-cloud
	@echo "==> Running cloud control-plane smoke test..."
	@./scripts/cloud_smoke.sh

cloud-stack-config:
	@ENV_FILE=deploy/cloud/.env; \
	if [ ! -f "$$ENV_FILE" ]; then ENV_FILE=deploy/cloud/.env.example; fi; \
	echo "==> Validating docker compose cloud stack with $$ENV_FILE..."; \
	docker compose --env-file "$$ENV_FILE" -f deploy/cloud/docker-compose.yml config > /dev/null

cloud-stack-smoke:
	@echo "==> Running docker-compose cloud stack smoke test..."
	@./deploy/cloud/smoke.sh

verify-beta: build-supported
	@./scripts/verify_beta.sh

verify-beta-docker: build-supported
	@./scripts/verify_beta.sh --docker

validate: build
	@echo "==> Running Codex validation scenario (Internal Version Bump)..."
	@./bin/orchestratorctl run start validation-run-01 validation-project || true
	@./bin/orchestratorctl submit validation-run-01 docs/validation_task.yaml
	@./bin/orchestratorctl step wait bump-version-01
