.PHONY: dev test lint build clean all

VERSION ?= 0.1.0
BINARY_NAME = heimdex-agent
BUILD_DIR = bin
LDFLAGS = -ldflags="-s -w -X main.Version=$(VERSION)"

# Default target
all: lint test build

# Run the agent in development mode
dev:
	@echo "Starting Heimdex Agent in development mode..."
	go run ./cmd/agent

# Run all tests with race detection
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Run linter (go vet + optional golangci-lint)
lint:
	@echo "Running linters..."
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi
	go fmt ./...

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/agent

# Build for multiple platforms
build-all: build-darwin build-windows

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/agent
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/agent

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/agent

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	go clean -cache -testcache

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Generate mocks (if needed in future)
generate:
	go generate ./...

# Run with specific log level
dev-debug:
	HEIMDEX_LOG_LEVEL=debug go run ./cmd/agent

# Check if the project compiles
check:
	go build ./...

.DEFAULT_GOAL := all
