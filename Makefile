.PHONY: all build build-darwin build-linux clean test test-coverage test-integration help

# Default target
all: build

# Build for current platform
build:
	@echo "Building for current platform..."
	go build -o one-mcp ./cmd/one-mcp

# Build for macOS (Darwin)
build-darwin:
	@echo "Building for macOS (Darwin)..."
	GOOS=darwin GOARCH=amd64 go build -o one-mcp-darwin ./cmd/one-mcp
	@echo "Built: one-mcp-darwin"

# Build for Linux
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o one-mcp-linux ./cmd/one-mcp
	@echo "Built: one-mcp-linux"

# Build for both platforms
build-all: build-darwin build-linux
	@echo "Built binaries for all platforms"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f one-mcp one-mcp-darwin one-mcp-linux one-mcp-test coverage.out
	@echo "Cleaned"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...
	@echo "Tests completed"

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "Coverage report generated: coverage.out"

# Run integration tests (builds binary first)
test-integration:
	@echo "Running integration tests..."
	@echo "Note: Binary will be built automatically by the test suite"
	go test -v -tags=integration ./internal/integration/...
	@echo "Integration tests completed"

# Show help
help:
	@echo "OneMCP Build Targets:"
	@echo "  make build            - Build for current platform"
	@echo "  make build-darwin     - Build for macOS"
	@echo "  make build-linux      - Build for Linux"
	@echo "  make build-all        - Build for all platforms"
	@echo "  make test             - Run unit tests"
	@echo "  make test-coverage    - Run tests with coverage report"
	@echo "  make test-integration - Run integration tests (builds binary first)"
	@echo "  make clean            - Remove build artifacts"
	@echo "  make help             - Show this help message"
