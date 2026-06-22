VERSION ?= v0.3.0-local-prod-rc.1
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DIRTY ?= $(shell test -z "$$(git status --porcelain 2>/dev/null)" && echo false || echo true)
LDFLAGS := -X agent-bridge/internal/app.Version=$(VERSION) -X agent-bridge/internal/buildinfo.Version=$(VERSION) -X agent-bridge/internal/buildinfo.Commit=$(COMMIT) -X agent-bridge/internal/buildinfo.Date=$(BUILD_DATE) -X agent-bridge/internal/buildinfo.Dirty=$(DIRTY)
TARGETS ?= darwin/arm64,darwin/amd64,linux/amd64
REQUIRE_TARGETS ?=
ALLOW_PARTIAL ?= 0
RELEASE_DOCKER_IMAGE ?= golang:1.25-bookworm
DIST ?= dist

all: lint test build

build-supported: build build-mcp-sdk-smoke

build-self-host-cloud: build-cloud

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
	@$(MAKE) build-gateway

build-codencer:
	@mkdir -p bin
	@echo "==> Building codencer..."
	@go build -ldflags "$(LDFLAGS)" -o bin/codencer ./cmd/codencer

build-gateway:
	@mkdir -p bin
	@echo "==> Building codencer-gatewayd..."
	@go build -ldflags "$(LDFLAGS)" -o bin/codencer-gatewayd ./cmd/codencer-gatewayd

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

verify-project-config: build-codencer
	@echo "==> Checking project config formatting..."
	@fmt=$$(gofmt -l internal/projectconfig internal/project internal/local cmd/codencer); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for project config files."; \
		exit 1; \
	fi
	@echo "==> Running project config unit tests..."
	@go test ./internal/projectconfig ./internal/project ./internal/local ./cmd/codencer
	@echo "==> Running project config CLI smoke..."
	@tmpdir=$$(mktemp -d "$${TMPDIR:-/tmp}/codencer-project-config.XXXXXX"); \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	home="$$tmpdir/home"; \
	repo="$$tmpdir/repo"; \
	scanrepo="$$tmpdir/scanrepo"; \
	missingrepo="$$tmpdir/missingrepo"; \
	mkdir -p "$$repo" "$$scanrepo" "$$missingrepo"; \
	git -C "$$repo" init -q; \
	git -C "$$missingrepo" init -q; \
	printf 'module example.test/scan\n' > "$$scanrepo/go.mod"; \
	before_scan=$$(find "$$scanrepo" -type f -print | sort); \
	CODENCER_HOME="$$home" ./bin/codencer init --json >/dev/null; \
	CODENCER_HOME="$$home" ./bin/codencer machine show --json >/dev/null; \
	CODENCER_HOME="$$home" ./bin/codencer machine set-label macbook-test --json >/dev/null; \
	CODENCER_HOME="$$home" ./bin/codencer project scan --repo "$$scanrepo" --json >/dev/null; \
	after_scan=$$(find "$$scanrepo" -type f -print | sort); \
	test "$$before_scan" = "$$after_scan"; \
	CODENCER_HOME="$$home" ./bin/codencer project init --repo "$$repo" --id test-project --name "Test Project" --json >/dev/null; \
	test -f "$$repo/.codencer/project.json"; \
	footprint=$$(find "$$repo/.codencer" -mindepth 1 -maxdepth 1 -print | wc -l | tr -d ' '); \
	test "$$footprint" = "1"; \
	before_project=$$(cat "$$repo/.codencer/project.json"); \
	CODENCER_HOME="$$home" ./bin/codencer project init --repo "$$repo" --json >/dev/null; \
	after_project=$$(cat "$$repo/.codencer/project.json"); \
	test "$$before_project" = "$$after_project"; \
	CODENCER_HOME="$$home" ./bin/codencer project adopt --repo "$$repo" --json >/dev/null; \
	if CODENCER_HOME="$$home" ./bin/codencer project adopt --repo "$$missingrepo" --json >/dev/null 2>&1; then \
		echo "ERROR: project adopt unexpectedly succeeded without .codencer/project.json"; \
		exit 1; \
	fi; \
	CODENCER_HOME="$$home" ./bin/codencer project list --json >/dev/null; \
	CODENCER_HOME="$$home" ./bin/codencer project status test-project --json >/dev/null

.PHONY: verify-docs-links
verify-docs-links:
	@python3 scripts/check_docs_links.py

.PHONY: verify-public-release
verify-public-release: verify-docs-links
	@echo "==> Checking public repository release boundary..."
	@python3 scripts/check_public_boundary.py

.PHONY: verify-public-selfhost-release
verify-public-selfhost-release: build build-mcp-sdk-smoke
	@echo "==> Running full Go test suite for public self-host release..."
	@go test ./...
	@echo "==> Creating release-like snapshot for public self-host release..."
	@$(MAKE) release-snapshot VERSION=v0.3.0-selfhost-verify
	@$(MAKE) verify-release-artifact-selfhost VERSION=v0.3.0-selfhost-artifact-verify TARGETS="$(TARGETS)" REQUIRE_TARGETS="$(REQUIRE_TARGETS)"
	@echo "==> Verifying public self-host config precedence and client setup artifacts..."
	@./scripts/verify_public_selfhost_release.sh
	@$(MAKE) verify-gateway
	@$(MAKE) verify-gateway-console
	@$(MAKE) verify-gateway-console-live
	@$(MAKE) verify-public-release

.PHONY: verify-public-selfhost-rc
verify-public-selfhost-rc:
	@./scripts/verify_public_selfhost_rc.sh

.PHONY: verify-gateway-console
verify-gateway-console:
	@echo "==> Installing Gateway Console dependencies..."
	@cd web/gateway-console && npm ci
	@echo "==> Checking Gateway Console formatting..."
	@cd web/gateway-console && npm run format:check
	@echo "==> Linting Gateway Console..."
	@cd web/gateway-console && npm run lint
	@echo "==> Typechecking Gateway Console..."
	@cd web/gateway-console && npm run typecheck
	@echo "==> Testing Gateway Console..."
	@cd web/gateway-console && npm run test
	@echo "==> Building Gateway Console..."
	@cd web/gateway-console && npm run build
	@echo "==> Running Gateway Console browser smoke..."
	@cd web/gateway-console && npm run test:e2e
	@echo "==> Capturing Gateway Console visual evidence..."
	@cd web/gateway-console && npm run visual:evidence

.PHONY: verify-gateway-console-live
verify-gateway-console-live: build
	@echo "==> Installing Gateway Console dependencies..."
	@cd web/gateway-console && npm ci
	@echo "==> Building Gateway Console for isolated live-mode verification..."
	@cd web/gateway-console && npm run build
	@echo "==> Running isolated Gateway Console live-mode browser verification..."
	@cd web/gateway-console && node tests/live/verify-live.mjs

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
	@$(MAKE) verify-project-config
	@$(MAKE) verify-local-relay-mcp
	@$(MAKE) verify-gateway
	@$(MAKE) verify-runtime-recovery
	@$(MAKE) verify-live-matrix
	@$(MAKE) activation-preflight
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

verify-gateway: build build-gateway
	@echo "==> Checking Gateway formatting..."
	@fmt=$$(gofmt -l internal/gateway cmd/codencer-gatewayd internal/setup internal/activation cmd/codencer); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for Gateway files."; \
		exit 1; \
	fi
	@echo "==> Running Gateway unit tests..."
	@go test ./internal/gateway ./internal/setup ./internal/activation ./cmd/codencer ./cmd/codencer-gatewayd
	@echo "==> Running Gateway E2E smoke..."
	@./scripts/verify_gateway.sh

verify-official-connector: build build-gateway
	@echo "==> Checking official connector formatting..."
	@fmt=$$(gofmt -l internal/account internal/gateway internal/connector internal/connectorops internal/setup cmd/codencer cmd/codencer-gatewayd); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for official connector files."; \
		exit 1; \
	fi
	@echo "==> Running official connector unit tests..."
	@go test ./internal/account ./internal/gateway ./internal/connector ./internal/connectorops ./internal/setup ./cmd/codencer ./cmd/codencer-gatewayd
	@echo "==> Running official connector isolated E2E..."
	@./scripts/verify_official_connector.sh

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

activation-preflight: build-codencer
	@echo "==> Running activation package/client preflight..."
	@tmpdir=$$(mktemp -d "$${TMPDIR:-/tmp}/codencer-activation.XXXXXX"); \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer activation check --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer activation package --relay https://relay.example.com --project codencer --token-env CODENCER_MCP_TOKEN --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer activation chatgpt --relay https://relay.example.com --project codencer --auth oauth --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer activation codex --relay https://relay.example.com --token-env CODENCER_MCP_TOKEN --json >/dev/null; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer activation claude-code --relay https://relay.example.com --token-env CODENCER_MCP_TOKEN --json >/dev/null

activation-package: build-codencer
	@tmpdir=$$(mktemp -d "$${TMPDIR:-/tmp}/codencer-activation-package.XXXXXX"); \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	CODENCER_HOME="$$tmpdir" ./bin/codencer activation package --relay "$${CODENCER_ACTIVATION_RELAY_URL:-https://relay.example.com}" --project "$${CODENCER_ACTIVATION_PROJECT:-codencer}" --token-env "$${CODENCER_ACTIVATION_TOKEN_ENV:-CODENCER_MCP_TOKEN}" --json

activation-relay-smoke: build-codencer
	@if [ -z "$${CODENCER_ACTIVATION_RELAY_URL:-}" ]; then \
		echo "SKIP: set CODENCER_ACTIVATION_RELAY_URL to run remote activation relay smoke."; \
		tmpdir=$$(mktemp -d "$${TMPDIR:-/tmp}/codencer-activation-smoke.XXXXXX"); \
		trap 'rm -rf "$$tmpdir"' EXIT; \
		CODENCER_HOME="$$tmpdir" ./bin/codencer activation check --json >/dev/null; \
	else \
		./bin/codencer activation check --relay "$${CODENCER_ACTIVATION_RELAY_URL}" --project "$${CODENCER_ACTIVATION_PROJECT:-}" --token-env "$${CODENCER_ACTIVATION_TOKEN_ENV:-CODENCER_MCP_TOKEN}" --check-oauth --json; \
	fi

activation-chatgpt-preflight: build-codencer
	@./bin/codencer activation chatgpt --relay "$${CODENCER_ACTIVATION_RELAY_URL:-https://relay.example.com}" --project "$${CODENCER_ACTIVATION_PROJECT:-codencer}" --auth oauth --json

activation-codex-mcp-preflight: build-codencer
	@./bin/codencer activation codex --relay "$${CODENCER_ACTIVATION_RELAY_URL:-https://relay.example.com}" --token-env "$${CODENCER_ACTIVATION_TOKEN_ENV:-CODENCER_MCP_TOKEN}" --json

activation-claude-code-mcp-preflight: build-codencer
	@./bin/codencer activation claude-code --relay "$${CODENCER_ACTIVATION_RELAY_URL:-https://relay.example.com}" --token-env "$${CODENCER_ACTIVATION_TOKEN_ENV:-CODENCER_MCP_TOKEN}" --json

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
	@partial_flag=""; \
	if [ "$(ALLOW_PARTIAL)" = "1" ]; then partial_flag="--allow-partial"; fi; \
	go run ./internal/release/cmd \
		--version "$(VERSION)" \
		--dist "$(DIST)" \
		--targets "$(TARGETS)" \
		--require-targets "$(REQUIRE_TARGETS)" \
		--docker-image "$(RELEASE_DOCKER_IMAGE)" \
		$$partial_flag \
		--json

.PHONY: verify-release-artifact-selfhost
verify-release-artifact-selfhost:
	@artifact_targets="$(TARGETS)"; \
	artifact_required="$(REQUIRE_TARGETS)"; \
	if [ "$$artifact_targets" = "darwin/arm64,darwin/amd64,linux/amd64" ]; then artifact_targets="host"; fi; \
	if [ -z "$$artifact_required" ]; then artifact_required="host"; fi; \
	VERSION="$(VERSION)" TARGETS="$$artifact_targets" REQUIRE_TARGETS="$$artifact_required" DIST="$(DIST)" ./scripts/verify_release_artifact_selfhost.sh

verify-release:
	@echo "==> Checking Sprint 6 formatting..."
	@fmt=$$(gofmt -l internal/buildinfo internal/security internal/setup internal/acceptance internal/proof internal/release internal/gateway cmd/codencer cmd/codencer-gatewayd internal/relay/router.go internal/relay/mcp_server.go internal/relay/mcp_server_test.go); \
	if [ -n "$$fmt" ]; then \
		echo "$$fmt"; \
		echo "ERROR: gofmt required for release hardening files."; \
		exit 1; \
	fi
	@echo "==> Running full Go test suite..."
	@go test ./...
	@$(MAKE) build
	@echo "==> Checking install/upgrade missing binary failures..."
	@missing=$$(mktemp -d "$${TMPDIR:-/tmp}/codencer-missing-bin.XXXXXX"); \
	trap 'rm -rf "$$missing"' EXIT; \
	if ./scripts/install.sh --bin-dir "$$missing" --dry-run --json >/dev/null 2>&1; then \
		echo "ERROR: install dry-run should fail when required binaries are missing."; \
		exit 1; \
	fi; \
	if ./scripts/upgrade.sh --bin-dir "$$missing" --dry-run --json >/dev/null 2>&1; then \
		echo "ERROR: upgrade dry-run should fail when required binaries are missing."; \
		exit 1; \
	fi
	@$(MAKE) release-snapshot VERSION=v0.3.0-local-prod-verify
	@$(MAKE) verify-local-execution
	@$(MAKE) verify-local-relay-mcp
	@$(MAKE) verify-gateway
	@$(MAKE) verify-runtime-recovery
	@$(MAKE) verify-live-matrix
	@$(MAKE) activation-preflight
	@$(MAKE) acceptance-local-production
	@$(MAKE) demo-local
	@./scripts/install.sh --dry-run --json >/dev/null
	@./scripts/uninstall.sh --dry-run --json >/dev/null
	@./scripts/upgrade.sh --dry-run --json >/dev/null

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

verify-supported-public: build-supported
	@./scripts/verify_supported_public.sh

verify-supported-public-docker: build-supported
	@./scripts/verify_supported_public.sh --docker

validate: build
	@echo "==> Running Codex validation scenario (Internal Version Bump)..."
	@./bin/orchestratorctl run start validation-run-01 validation-project || true
	@./bin/orchestratorctl submit validation-run-01 docs/validation_task.yaml
	@./bin/orchestratorctl step wait bump-version-01
