.PHONY: all build build-darwin build-linux build-all clean test test-coverage test-integration help

all: build ## Build for current platform (default target)

build: ## Build for current platform
	@echo "Building for current platform..."
	go build -o one-mcp ./cmd/one-mcp

build-darwin: ## Build for macOS (Darwin)
	@echo "Building for macOS (Darwin)..."
	GOOS=darwin GOARCH=amd64 go build -o one-mcp-darwin ./cmd/one-mcp
	@echo "Built: one-mcp-darwin"

build-linux: ## Build for Linux
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o one-mcp-linux ./cmd/one-mcp
	@echo "Built: one-mcp-linux"

build-all: build-darwin build-linux ## Build for all platforms (macOS and Linux)
	@echo "Built binaries for all platforms"

clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	rm -f one-mcp one-mcp-darwin one-mcp-linux one-mcp-test coverage.out
	@echo "Cleaned"

test: ## Run unit tests
	@echo "Running tests..."
	go test -v ./...
	@echo "Tests completed"

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "Coverage report generated: coverage.out"

test-integration: ## Run integration tests (builds binary first)
	@echo "Running integration tests..."
	@echo "Note: Binary will be built automatically by the test suite"
	go test -v -tags=integration ./integration/...
	@echo "Integration tests completed"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
