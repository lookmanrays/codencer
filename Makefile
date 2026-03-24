.PHONY: all build test lint run

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
