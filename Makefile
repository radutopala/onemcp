.PHONY: all build build-darwin build-linux clean test test-coverage help

# Default target
all: build

# Build for current platform
build:
	@echo "Building for current platform..."
	go build -o one-mcp-server ./cmd/one-mcp-server

# Build for macOS (Darwin)
build-darwin:
	@echo "Building for macOS (Darwin)..."
	GOOS=darwin GOARCH=amd64 go build -o one-mcp-server-darwin ./cmd/one-mcp-server
	@echo "Built: one-mcp-server-darwin"

# Build for Linux
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o one-mcp-server-linux ./cmd/one-mcp-server
	@echo "Built: one-mcp-server-linux"

# Build for both platforms
build-all: build-darwin build-linux
	@echo "Built binaries for all platforms"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f one-mcp-server one-mcp-server-darwin one-mcp-server-linux coverage.out
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

# Show help
help:
	@echo "OneMCP Build Targets:"
	@echo "  make build         - Build for current platform"
	@echo "  make build-darwin  - Build for macOS"
	@echo "  make build-linux   - Build for Linux"
	@echo "  make build-all     - Build for all platforms"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make help          - Show this help message"
