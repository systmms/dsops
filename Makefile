# Variables
BINARY_NAME := dsops
BUILD_DIR := ./bin
MAIN_PATH := ./cmd/dsops
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u '+%Y-%m-%d_%I:%M:%S%p')

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -w -s"

# Default target
.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: setup
setup: ## Set up development environment
	@echo "Setting up development environment..."
	go mod download
	go mod tidy
	@if [ -n "$$DSOPS_DEV" ]; then \
		echo "Nix development environment detected - tools already available"; \
	else \
		echo "Installing development tools..."; \
		command -v golangci-lint >/dev/null 2>&1 || { \
			echo "Installing golangci-lint..."; \
			curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.7.2; \
		}; \
	fi
	@echo "Setup complete!"

.PHONY: build
build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: install
install: ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) $(MAIN_PATH)
	@echo "Installed $(BINARY_NAME) to $(shell go env GOPATH)/bin"

.PHONY: dev
dev: build ## Build and run for development
	@echo "Running $(BINARY_NAME) in development mode..."
	$(BUILD_DIR)/$(BINARY_NAME) --debug

.PHONY: run
run: build ## Build and run dsops with arguments (use ARGS="...")
	@echo "Running $(BINARY_NAME)..."
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

.PHONY: test
test: test-short ## Run unit tests (alias for test-short)

.PHONY: test-short
test-short: ## Run unit tests only (fast, no integration tests)
	@echo "Running unit tests..."
	go test -short -race -v ./internal/... ./pkg/... ./cmd/...

.PHONY: test-race
test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	go test -race -short -v ./internal/... ./pkg/... ./cmd/...

.PHONY: test-integration
test-integration: ## Run integration tests (requires Docker)
	@echo "Running integration tests..."
	@if ! command -v docker >/dev/null 2>&1; then \
		echo "Error: Docker not found. Please install Docker to run integration tests."; \
		exit 1; \
	fi
	@echo "Running integration tests (tests manage their own Docker containers)..."
	@go test -race -v -timeout=300s -p=1 ./tests/integration/...
	@echo "Integration tests complete!"

.PHONY: test-all
test-all: ## Run all tests (unit + integration + race detection)
	@echo "Running all tests..."
	go test -race -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.txt -covermode=atomic ./internal/... ./pkg/... ./cmd/...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@echo ""
	@echo "Coverage summary:"
	@go tool cover -func=coverage.txt | grep total || true

.PHONY: coverage-report
coverage-report: ## Generate HTML coverage report from existing coverage.txt
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report: coverage.html"
	@which open > /dev/null && open coverage.html || true

.PHONY: lint
lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	goimports -w -local github.com/systmms/dsops .

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean

# Release targets (for CI)
.PHONY: build-all
build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)

.PHONY: release
release: clean test build-all ## Create a release (clean, test, build all platforms)
	@echo "Release build complete!"
	@echo "Binaries available in $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/

.PHONY: check
check: lint vet test ## Run all checks (lint, vet, test)

.PHONY: ci
ci: check build ## Run CI pipeline (check + build)

# Documentation targets
.PHONY: docs
docs: docs-build ## Build documentation (alias for docs-build)

.PHONY: docs-install
docs-install: ## Install documentation dependencies
	@echo "Installing documentation dependencies..."
	cd docs && npm install

.PHONY: docs-serve
docs-serve: ## Serve documentation locally
	@echo "Starting documentation server..."
	cd docs && npm run dev

.PHONY: docs-build
docs-build: ## Build documentation site
	@echo "Building documentation..."
	cd docs && npm run build

.PHONY: docs-clean
docs-clean: ## Clean documentation build
	@echo "Cleaning documentation build..."
	cd docs && rm -rf public resources node_modules

# Development helpers
.PHONY: watch
watch: ## Watch for changes and rebuild
	@echo "Watching for changes..."
	@command -v entr >/dev/null 2>&1 || { echo "entr not found. Install with: brew install entr"; exit 1; }
	find . -name '*.go' | entr -r make build