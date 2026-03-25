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

setup:
	@echo "==> Initializing local environment..."
	@mkdir -p bin
	@mkdir -p .codencer/artifacts
	@mkdir -p .codencer/workspace

clean:
	@echo "==> Cleaning up..."
	@rm -rf bin
	@rm -rf .codencer/workspace/*
	@rm -f codencer.db

simulate: build
	@echo "==> Running in simulation mode (all adapters stubbed)..."
	@ALL_ADAPTERS_SIMULATION_MODE=1 ./bin/orchestratord
