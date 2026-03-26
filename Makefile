all: lint test build

build:
	@echo "==> Building orchestratord..."
	@go build -ldflags "-X agent-bridge/internal/app.Version=v0.0.1" -o bin/orchestratord ./cmd/orchestratord
	@echo "==> Building orchestratorctl..."
	@go build -ldflags "-X agent-bridge/internal/app.Version=v0.0.1" -o bin/orchestratorctl ./cmd/orchestratorctl

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

setup:
	@echo "==> Initializing local environment (.codencer/)..."
	@mkdir -p bin
	@mkdir -p .codencer/artifacts
	@mkdir -p .codencer/workspace

doctor: build
	@echo "==> Verifying local environment..."
	@ls -d .codencer > /dev/null 2>&1 || echo "WARNING: .codencer directory missing. Run 'make setup'"
	@which go > /dev/null 2>&1 || echo "ERROR: go not found"
	@echo "Checking adapter binaries..."
	@which codex-agent > /dev/null 2>&1 || echo "INFO: codex-agent not found (Simulation Mode or CODEX_BINARY required for real use)"
	@echo "Ready for local development."

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
	@./bin/orchestratorctl run start validation-run-01 validation-project --force || true
	@./bin/orchestratorctl submit validation-run-01 docs/validation_task.yaml
	@./bin/orchestratorctl step wait bump-version-01
